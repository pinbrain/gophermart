package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/pinbrain/gophermart/internal/appctx"
	"github.com/pinbrain/gophermart/internal/logger"
	"github.com/pinbrain/gophermart/internal/middleware"
	"github.com/pinbrain/gophermart/internal/model"
	"github.com/pinbrain/gophermart/internal/storage"
	"github.com/pinbrain/gophermart/internal/utils"
)

type UserHandler struct {
	storage Storage
}

type Storage interface {
	CreateUser(ctx context.Context, login, password string) (int, error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
	CreateOrder(ctx context.Context, userID int, orderNum string) (int, error)
	GetUserOrders(ctx context.Context, userID int) ([]model.Order, error)
	GetUserBalance(ctx context.Context, userID int) (*model.Balance, error)
	Withdraw(ctx context.Context, userID int, sum float64, order string) error
	GetWithdrawals(ctx context.Context, userID int) ([]model.Withdrawn, error)
	Close()
}

func newUserHandler(storage Storage) UserHandler {
	return UserHandler{storage: storage}
}

func (h *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		http.Error(w, "Некорректный Content-Type", http.StatusBadRequest)
		return
	}

	var user model.User
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&user); err != nil {
		logger.Log.WithError(err).Error("failed to decode register user req body")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user.Login == "" || user.Password == "" {
		http.Error(w, "Не все обязательные поля заполнены", http.StatusBadRequest)
		return
	}

	userID, err := h.storage.CreateUser(r.Context(), user.Login, user.Password)
	if err != nil {
		if errors.Is(err, storage.ErrLoginTaken) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		logger.Log.WithError(err).Error("failed to register new user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	user.ID = userID
	jwtString, err := utils.BuildJWTSting(user)
	if err != nil {
		logger.Log.WithError(err).Error("failed to register new user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	middleware.SetJWTCookie(w, jwtString)

	w.WriteHeader(200)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		http.Error(w, "Некорректный Content-Type", http.StatusBadRequest)
		return
	}

	var reqUser model.User
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reqUser); err != nil {
		logger.Log.WithError(err).Error("failed to decode register user req body")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if reqUser.Login == "" || reqUser.Password == "" {
		http.Error(w, "Не все обязательные поля заполнены", http.StatusBadRequest)
		return
	}

	dbUser, err := h.storage.GetUserByLogin(r.Context(), reqUser.Login)
	if err != nil {
		if errors.Is(err, storage.ErrNoUser) {
			logger.Log.WithError(err).Debug()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		logger.Log.WithError(err).Error("failed to login user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if isPwdOk := utils.ComparePwdAndHash(reqUser.Password, dbUser.PasswordHash); !isPwdOk {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	jwtString, err := utils.BuildJWTSting(*dbUser)
	if err != nil {
		logger.Log.WithError(err).Error("failed to login user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	middleware.SetJWTCookie(w, jwtString)

	w.WriteHeader(200)
}

func (h *UserHandler) CreateNewOrder(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		http.Error(w, "Некорректный Content-Type", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.WithError(err).Error("failed to read request order num")
		http.Error(w, "Не удалось прочитать номер заказа запросе", http.StatusInternalServerError)
		return
	}
	orderNum := string(body)
	if !utils.IsValidOrderNum(orderNum) {
		http.Error(w, "Некорректный номер заказа", http.StatusUnprocessableEntity)
		return
	}
	user := appctx.GetCtxUser(r.Context())
	_, err = h.storage.CreateOrder(r.Context(), user.ID, orderNum)
	if err != nil {
		if errors.Is(err, storage.ErrOrderNumUsed) {
			logger.Log.WithError(err).Debug()
			w.WriteHeader(http.StatusConflict)
			return
		}
		if errors.Is(err, storage.ErrOrderNumCreated) {
			w.WriteHeader(http.StatusOK)
			return
		}
		logger.Log.WithError(err).Error("failed to create new order")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *UserHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	user := appctx.GetCtxUser(r.Context())
	orders, err := h.storage.GetUserOrders(r.Context(), user.ID)
	if err != nil {
		logger.Log.WithError(err).Error("failed to read user orders")
		http.Error(w, "Не удалось получить заказы", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	enc := json.NewEncoder(w)
	if err = enc.Encode(orders); err != nil {
		logger.Log.WithError(err).Error("Error in encoding user orders response to json")
	}
}

func (h *UserHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	user := appctx.GetCtxUser(r.Context())
	balance, err := h.storage.GetUserBalance(r.Context(), user.ID)
	if err != nil {
		logger.Log.WithError(err).Error("failed to read user balance")
		http.Error(w, "Не удалось получить баланс пользователя", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err = enc.Encode(balance); err != nil {
		logger.Log.WithError(err).Error("Error in encoding user balance response to json")
	}
}

func (h *UserHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		http.Error(w, "Некорректный Content-Type", http.StatusBadRequest)
		return
	}

	var reqWithdraw model.Withdrawn
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reqWithdraw); err != nil {
		logger.Log.WithError(err).Error("failed to decode withdraw req body")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if reqWithdraw.Number == "" || !utils.IsValidOrderNum(reqWithdraw.Number) {
		http.Error(w, "Некорректный номер заказа", http.StatusUnprocessableEntity)
		return
	}
	if reqWithdraw.Sum <= 0 {
		http.Error(w, "Некорректная сумма для списания", http.StatusBadRequest)
		return
	}

	user := appctx.GetCtxUser(r.Context())
	err := h.storage.Withdraw(r.Context(), user.ID, reqWithdraw.Sum, reqWithdraw.Number)
	if err != nil {
		if errors.Is(err, storage.ErrInsufficientFunds) {
			logger.Log.WithError(err).Debug()
			http.Error(w, "Недостаточно средств на счету", http.StatusPaymentRequired)
			return
		}
		if errors.Is(err, storage.ErrOrderNumUsed) {
			logger.Log.WithError(err).Debug()
			http.Error(w, "Номер заказа уже был использован", http.StatusConflict)
			return
		}
		logger.Log.WithError(err).Error("failed to withdraw")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *UserHandler) GetWithdraws(w http.ResponseWriter, r *http.Request) {
	user := appctx.GetCtxUser(r.Context())
	withdrawals, err := h.storage.GetWithdrawals(r.Context(), user.ID)
	if err != nil {
		logger.Log.WithError(err).Error("failed to read user withdrawals")
		http.Error(w, "Не удалось получить информацию о выводе средств", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	enc := json.NewEncoder(w)
	if err = enc.Encode(withdrawals); err != nil {
		logger.Log.WithError(err).Error("Error in encoding user withdrawals response to json")
	}
}
