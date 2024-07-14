package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pinbrain/gophermart/internal/logger"
	"github.com/pinbrain/gophermart/internal/model"
)

const (
	// Интервал проверки наличия необработанных заказов
	checkInterval = 10 * time.Second
	// Количество горутин, отправляющих запросы в accrual
	workerCount = 5
)

var ErrReqLimit = errors.New("too many requests")

type Storage interface {
	GetOrdersToProcess(ctx context.Context) ([]model.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int, status model.OrderStatus, accrual float64) error
}

type AccrualAgent struct {
	appCtx     context.Context
	storage    Storage
	accrualURL string

	rateLimit        sync.RWMutex
	rateLimitEndTime time.Time
}

func NewAccrualAgent(ctx context.Context, storage Storage, accrualURL string) AccrualAgent {
	return AccrualAgent{
		appCtx:     ctx,
		storage:    storage,
		accrualURL: accrualURL,

		rateLimit:        sync.RWMutex{},
		rateLimitEndTime: time.Time{},
	}
}

func (aa *AccrualAgent) fetchOrderStatus(ctx context.Context, orderNum string) (*model.AccrualResultRes, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/orders/%s", aa.accrualURL, orderNum), nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		retryAfter := res.Header.Get("Retry-After")
		retryAfterDuration, err := time.ParseDuration(retryAfter + "s")
		if err != nil {
			return nil, err
		}

		aa.rateLimit.Lock()
		aa.rateLimitEndTime = time.Now().Add(retryAfterDuration)
		aa.rateLimit.Unlock()

		return nil, ErrReqLimit
	}

	if res.StatusCode == http.StatusNoContent {
		logger.Log.WithField("orderNum", orderNum).Infoln("Order is not registered in accrual system")
		return &model.AccrualResultRes{
			Order:  orderNum,
			Status: model.ORDER_ACC_INVALID,
		}, nil
	}

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			body = []byte("failed to read response body")
		}
		return nil, fmt.Errorf("error response from accrual service with status code %d: %s", res.StatusCode, body)
	}

	var result model.AccrualResultRes
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (aa *AccrualAgent) worker(ctx context.Context, id int, ordersCh <-chan model.Order) {
	workerLogger := logger.Log.WithField("workerID", id)
	for {
		select {
		case <-ctx.Done():
			workerLogger.Debug("Worker stopped")
			return
		case order, ok := <-ordersCh:
			if !ok {
				return
			}
			workerLogger.Debugf("going to process order #%s", order.Number)

			aa.rateLimit.RLock()
			sleepDuration := time.Until(aa.rateLimitEndTime)
			aa.rateLimit.RUnlock()

			if sleepDuration > 0 {
				workerLogger.Debugf("Rate limited, sleeping for %s", sleepDuration)
				// Пока горутина ждет таймаут может прийти сигнал о завершении работы, на который нужно среагировать
				select {
				case <-ctx.Done():
					workerLogger.Debug("Worker stopped from sleeping state")
					return
				case <-time.After(sleepDuration):
				}
			}

			result, err := aa.fetchOrderStatus(ctx, order.Number)
			if err != nil {
				if errors.Is(err, ErrReqLimit) {
					workerLogger.Info("Accrual service request limit reached")
				} else {
					workerLogger.WithError(err).Error("error fetching order status")
				}
				continue
			}
			orderStatus := model.ORDER_PROCESSING
			switch result.Status {
			case model.ORDER_ACC_PROCESSED:
				orderStatus = model.ORDER_PROCESSED
			case model.ORDER_ACC_INVALID:
				orderStatus = model.ORDER_INVALID
			}
			if err := aa.storage.UpdateOrderStatus(ctx, order.ID, orderStatus, result.Accrual); err != nil {
				workerLogger.WithError(err).Error("error updating order process status")
			}
		}
	}
}

func (aa *AccrualAgent) processOrders(ctx context.Context, ordersCh chan<- model.Order, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("Process order stopped")
			return
		case <-time.After(checkInterval):
			orders, err := aa.storage.GetOrdersToProcess(aa.appCtx)
			if err != nil {
				logger.Log.WithError(err).Error("failed to get orders to process from storage")
				continue
			}
			for _, order := range orders {
				select {
				case <-aa.appCtx.Done():
					return
				case ordersCh <- order:
				}
			}
		}
	}
}

func (aa *AccrualAgent) StartAgent(wg *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ordersCh := make(chan model.Order, workerCount)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			aa.worker(ctx, id, ordersCh)
		}(i)
	}

	wg.Add(1)
	go aa.processOrders(ctx, ordersCh, wg)

	<-aa.appCtx.Done()
	logger.Log.Debug("Stopping accrual agent workers...")
	cancel()
	close(ordersCh)
}
