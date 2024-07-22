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
	storage    Storage
	accrualURL string

	ctx       context.Context
	ctxCancel context.CancelFunc
	ordersCh  chan model.Order
	wg        sync.WaitGroup

	rateLimit        sync.RWMutex
	rateLimitEndTime time.Time
}

func NewAccrualAgent(storage Storage, accrualURL string) *AccrualAgent {
	return &AccrualAgent{
		storage:    storage,
		accrualURL: accrualURL,

		wg:               sync.WaitGroup{},
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
			Status: model.OrderAccInvalid,
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

func (aa *AccrualAgent) worker(id int, ordersCh <-chan model.Order) {
	workerLogger := logger.Log.WithField("workerID", id)
	for {
		select {
		case <-aa.ctx.Done():
			workerLogger.Debug("Worker stopped")
			return
		case order, ok := <-ordersCh:
			if !ok {
				return
			}
			workerLogger.Debugf("going to process order #%s", order.Number)
			for {
				aa.rateLimit.RLock()
				sleepDuration := time.Until(aa.rateLimitEndTime)
				aa.rateLimit.RUnlock()

				if sleepDuration > 0 {
					workerLogger.Debugf("Rate limited, sleeping for %s", sleepDuration)
					// Пока горутина ждет таймаут может прийти сигнал о завершении работы, на который нужно среагировать
					select {
					case <-aa.ctx.Done():
						workerLogger.Debug("Worker stopped from sleeping state")
						return
					case <-time.After(sleepDuration):
					}
				}
				workerLogger.Debugf("fetching order #%s", order.Number)
				result, err := aa.fetchOrderStatus(aa.ctx, order.Number)
				if err != nil {
					if errors.Is(err, ErrReqLimit) {
						workerLogger.Info("Accrual service request limit reached")
						continue
					} else {
						workerLogger.WithError(err).Error("error fetching order status")
						break
					}
				}
				orderStatus := model.OrderProcessing
				switch result.Status {
				case model.OrderAccProcessed:
					orderStatus = model.OrderProcessed
				case model.OrderAccInvalid:
					orderStatus = model.OrderInvalid
				}
				if err := aa.storage.UpdateOrderStatus(aa.ctx, order.ID, orderStatus, result.Accrual); err != nil {
					workerLogger.WithError(err).Error("error updating order process status")
				}
				break
			}
		}
	}
}

func (aa *AccrualAgent) processOrders(ordersCh chan<- model.Order) {
	defer aa.wg.Done()
	for {
		select {
		case <-aa.ctx.Done():
			logger.Log.Debug("Process order stopped")
			return
		case <-time.After(checkInterval):
			orders, err := aa.storage.GetOrdersToProcess(aa.ctx)
			if err != nil {
				logger.Log.WithError(err).Error("failed to get orders to process from storage")
				continue
			}
			for _, order := range orders {
				select {
				case <-aa.ctx.Done():
					logger.Log.Debug("Process order stopped (while adding orders to chanel)")
					return
				case ordersCh <- order:
				}
			}
		}
	}
}

func (aa *AccrualAgent) StartAgent() {
	aa.ctx, aa.ctxCancel = context.WithCancel(context.Background())

	aa.ordersCh = make(chan model.Order, workerCount)

	for i := 0; i < workerCount; i++ {
		aa.wg.Add(1)
		go func(id int) {
			defer aa.wg.Done()
			aa.worker(id, aa.ordersCh)
		}(i)
	}

	aa.wg.Add(1)
	go aa.processOrders(aa.ordersCh)
}

func (aa *AccrualAgent) StopAgent() {
	// Если агент уже завершил работу, то ничего не делаем
	if err := aa.ctx.Err(); err != nil {
		logger.Log.Debug("Accrual agent already stopped")
		return
	}
	logger.Log.Debug("Stopping accrual agent workers...")
	aa.ctxCancel()
	close(aa.ordersCh)
	aa.wg.Wait()
}
