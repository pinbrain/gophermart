-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  login VARCHAR UNIQUE NOT NULL,
  password_hash VARCHAR NOT NULL
);
COMMENT ON COLUMN users.login IS 'Логин пользователя';
COMMENT ON COLUMN users.password_hash IS 'Хэш пароля пользователя';

CREATE TABLE orders (
  id SERIAL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users (id),
  number VARCHAR UNIQUE NOT NULL,
  status VARCHAR(10) NOT NULL,
  accrual FLOAT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON COLUMN orders.user_id IS 'Id пользователя заказа';
COMMENT ON COLUMN orders.number IS 'Номер заказа';
COMMENT ON COLUMN orders.status IS 'Статус обработки заказа в системе расчета начислений баллов';
COMMENT ON COLUMN orders.accrual IS 'Сумма начисленных баллов по заказу';
COMMENT ON COLUMN orders.created_at IS 'Timestamp создания записи';
COMMENT ON COLUMN orders.updated_at IS 'Timestamp обновления записи';

CREATE TABLE withdrawals (
  id SERIAL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users (id),
  number VARCHAR UNIQUE NOT NULL,
  sum FLOAT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON COLUMN withdrawals.user_id IS 'Id пользователя списания баллов';
COMMENT ON COLUMN withdrawals.number IS 'Номер заказа';
COMMENT ON COLUMN withdrawals.sum IS 'Сумма списанных баллов по заказу';
COMMENT ON COLUMN withdrawals.created_at IS 'Timestamp создания записи';

CREATE TABLE balances (
  user_id INT NOT NULL UNIQUE REFERENCES users (id),
  current FLOAT NOT NULL DEFAULT 0,
  withdrawn FLOAT NOT NULL DEFAULT 0
);
COMMENT ON COLUMN balances.user_id IS 'Id пользователя баланса';
COMMENT ON COLUMN balances.current IS 'Текущий баланс (сумма баллов) пользователя';
COMMENT ON COLUMN balances.withdrawn IS 'Суммарное количество списанных баллов за все время';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE orders;
DROP TABLE withdrawals;
DROP TABLE balances;
DROP TABLE users;
-- +goose StatementEnd
