package model

import (
	"encoding/json"
	"time"
)

type OrderAccrualStatus string

// Возможные статусы обработки заказов в части начисления бонусных баллов
const (
	ORDER_ACC_REGISTERED OrderAccrualStatus = "REGISTERED"
	ORDER_ACC_PROCESSING OrderAccrualStatus = "PROCESSING"
	ORDER_ACC_INVALID    OrderAccrualStatus = "INVALID"
	ORDER_ACC_PROCESSED  OrderAccrualStatus = "PROCESSED"
)

type OrderStatus string

// Возможные статусы обработки заказов в системе (внутренние)
const (
	ORDER_NEW        OrderStatus = "NEW"
	ORDER_PROCESSING OrderStatus = "PROCESSING"
	ORDER_INVALID    OrderStatus = "INVALID"
	ORDER_PROCESSED  OrderStatus = "PROCESSED"
)

type User struct {
	ID           int    `json:"-"`
	Login        string `json:"login"`
	PasswordHash string `json:"-"`
	Password     string `json:"password"`
}

// Заказ для начисления бонусных баллов
type Order struct {
	ID        int         `json:"-"`
	UserID    int         `json:"-"`
	Number    string      `json:"number"`
	Status    OrderStatus `json:"status"`
	Accrual   float64     `json:"accrual,omitempty"`
	CreatedAt time.Time   `json:"uploaded_at"`
	UpdatedAt time.Time   `json:"-"`
}

func (o Order) MarshalJSON() ([]byte, error) {
	type OrderAlias Order

	aliasValue := struct {
		OrderAlias
		UploadedAt string `json:"uploaded_at"`
	}{
		OrderAlias: OrderAlias(o),
		UploadedAt: o.CreatedAt.Format(time.RFC3339),
	}

	return json.Marshal(aliasValue)
}

// Заказы для списания бонусных баллов
type Withdrawn struct {
	ID        int       `json:"-"`
	UserID    int       `json:"-"`
	Number    string    `json:"order"`
	Sum       float64   `json:"sum"`
	CreatedAt time.Time `json:"processed_at"`
}

func (w Withdrawn) MarshalJSON() ([]byte, error) {
	type WithdrawnAlias Withdrawn

	aliasValue := struct {
		WithdrawnAlias
		CreatedAt string `json:"processed_at"`
	}{
		WithdrawnAlias: WithdrawnAlias(w),
		CreatedAt:      w.CreatedAt.Format(time.RFC3339),
	}

	return json.Marshal(aliasValue)
}

// Текущий баланс пользователя
type Balance struct {
	UserID    int     `json:"-"`
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
