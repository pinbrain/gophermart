package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pinbrain/gophermart/internal/model"
	"github.com/pinbrain/gophermart/internal/utils"
)

var (
	ErrLoginTaken        = errors.New("login is already taken")
	ErrNoUser            = errors.New("user not found in db")
	ErrOrderNumUsed      = errors.New("order num is already registered by another user")
	ErrOrderNumCreated   = errors.New("order num is already registered by user")
	ErrInsufficientFunds = errors.New("insufficient funds in the account")
)

type DBStorage struct {
	db *DB
}

type StorageCfg struct {
	DSN string
}

func NewStorage(ctx context.Context, cfg StorageCfg) (*DBStorage, error) {
	db, err := newDB(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}
	storage := DBStorage{db: db}
	return &storage, nil
}

func (st *DBStorage) Close() {
	st.db.pool.Close()
}

func (st *DBStorage) CreateUser(ctx context.Context, login, password string) (int, error) {
	login = strings.ToLower(login)
	passwordHash, err := utils.GeneratePasswordHash(password)
	if err != nil {
		return 0, fmt.Errorf("failed to create new user: %w", err)
	}
	tx, err := st.db.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create new user: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO users (login, password_hash)
		VALUES ($1, $2) RETURNING id;`, login, passwordHash,
	)
	var userID int
	err = row.Scan(&userID)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) {
			if pgError.Code == pgerrcode.UniqueViolation {
				return 0, ErrLoginTaken
			}
		}
		return 0, fmt.Errorf("failed to create new user: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO balances (user_id) VALUES ($1);`, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create new user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to create new user: %w", err)
	}
	return userID, nil
}

func (st *DBStorage) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	login = strings.ToLower(login)
	user := model.User{
		Login: login,
	}
	row := st.db.pool.QueryRow(ctx, `
		SELECT id, password_hash FROM users WHERE login = $1`, login,
	)
	if err := row.Scan(&user.ID, &user.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoUser
		}
		return nil, fmt.Errorf("failed to get user from db: %w", err)
	}
	return &user, nil
}

func (st *DBStorage) CreateOrder(ctx context.Context, userID int, orderNum string) (int, error) {
	row := st.db.pool.QueryRow(ctx, `
		INSERT INTO orders (user_id, number, status) VALUES ($1, $2, $3) RETURNING id`,
		userID, orderNum, model.ORDER_NEW,
	)
	var orderID int
	err := row.Scan(&orderID)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) {
			if pgError.Code == pgerrcode.UniqueViolation {
				order, err := st.GetOrderByNum(ctx, orderNum)
				if err == nil {
					if order.UserID == userID {
						return orderID, ErrOrderNumCreated
					}
					return orderID, ErrOrderNumUsed
				}
			}
		}
		return 0, fmt.Errorf("failed to create new order: %w", err)
	}
	return orderID, nil
}

func (st *DBStorage) GetOrderByNum(ctx context.Context, orderNum string) (*model.Order, error) {
	row := st.db.pool.QueryRow(ctx, `
		SELECT
			id,
			user_id,
			number,
			status,
			COALESCE(accrual, 0),
			created_at,
			updated_at
		FROM orders WHERE number = $1`,
		orderNum,
	)
	var order model.Order
	err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.Number,
		&order.Status,
		&order.Accrual,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get order by number: %w", err)
	}
	return &order, nil
}

func (st *DBStorage) GetUserOrders(ctx context.Context, userID int) ([]model.Order, error) {
	orders := []model.Order{}
	rows, err := st.db.pool.Query(ctx, `
		SELECT
			id,
			user_id,
			number,
			status,
			COALESCE(accrual, 0),
			created_at,
			updated_at
		FROM orders WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select user orders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var order model.Order
		if err = rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Number,
			&order.Status,
			&order.Accrual,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to read data from db order row: %w", err)
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to select user orders: %w", err)
	}

	return orders, nil
}

func (st *DBStorage) GetUserBalance(ctx context.Context, userID int) (*model.Balance, error) {
	row := st.db.pool.QueryRow(ctx, `
		SELECT current, withdrawn FROM balances WHERE user_id = $1;`,
		userID,
	)
	var balance model.Balance
	if err := row.Scan(&balance.Current, &balance.Withdrawn); err != nil {
		return nil, fmt.Errorf("failed to get user balance: %w", err)
	}
	balance.UserID = userID
	return &balance, nil
}

func (st *DBStorage) Withdraw(ctx context.Context, userID int, sum float64, order string) error {
	tx, err := st.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT current FROM balances WHERE user_id = $1 FOR UPDATE;`,
		userID,
	)
	var current float64
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	if current < sum {
		return ErrInsufficientFunds
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO withdrawals (user_id, number, sum) VALUES ($1, $2, $3);`,
		userID, order, sum,
	)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) {
			if pgError.Code == pgerrcode.UniqueViolation {
				return ErrOrderNumUsed
			}
		}
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE balances SET current = current - $1, withdrawn = withdrawn + $1 WHERE user_id = $2`,
		sum, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	return nil
}

func (st *DBStorage) GetWithdrawals(ctx context.Context, userID int) ([]model.Withdrawn, error) {
	withdrawals := []model.Withdrawn{}
	rows, err := st.db.pool.Query(ctx, `
		SELECT
			id,
			user_id,
			number,
			sum,
			created_at
		FROM withdrawals WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select user withdrawals: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var withdrawn model.Withdrawn
		if err = rows.Scan(
			&withdrawn.ID,
			&withdrawn.UserID,
			&withdrawn.Number,
			&withdrawn.Sum,
			&withdrawn.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to read data from db withdrawn row: %w", err)
		}
		withdrawals = append(withdrawals, withdrawn)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to select user withdrawals: %w", err)
	}

	return withdrawals, nil
}

func (st *DBStorage) GetOrdersToProcess(ctx context.Context) ([]model.Order, error) {
	orders := []model.Order{}
	rows, err := st.db.pool.Query(ctx, `
		SELECT
			id,
			user_id,
			number,
			status,
			COALESCE(accrual, 0),
			created_at,
			updated_at
		FROM orders WHERE status IN ($1, $2)`,
		model.ORDER_NEW, model.ORDER_PROCESSING,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select orders for processing: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var order model.Order
		if err = rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Number,
			&order.Status,
			&order.Accrual,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to read data from db order row: %w", err)
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to select orders for processing: %w", err)
	}

	return orders, nil
}

func (st *DBStorage) UpdateOrderStatus(ctx context.Context, orderID int, status model.OrderStatus, accrual float64) error {
	tx, err := st.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT user_id FROM orders WHERE id = $1;`,
		orderID,
	)
	var userID int
	if err := row.Scan(&userID); err != nil {
		return fmt.Errorf("there is no order with id = %d: %w", orderID, err)
	}

	var accrualToUpdate *float64
	if accrual > 0 {
		accrualToUpdate = &accrual
		_, err = tx.Exec(ctx, `UPDATE balances SET current = current + $1 WHERE user_id = $2`,
			accrual, userID,
		)
		if err != nil {
			return fmt.Errorf("failed to update order status: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE orders SET status = $1, accrual = $2, updated_at = NOW() WHERE id = $3`,
		status, accrualToUpdate, orderID,
	)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}
