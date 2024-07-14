package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pinbrain/gophermart/internal/handlers/mocks"
	"github.com/pinbrain/gophermart/internal/middleware"
	"github.com/pinbrain/gophermart/internal/model"
	"github.com/pinbrain/gophermart/internal/storage"
	"github.com/pinbrain/gophermart/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
	}
	type request struct {
		body        string
		contentType string
	}
	type storageRes struct {
		userID int
		err    error
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
	}{
		{
			name: "Корректный запрос",
			request: request{
				body:        `{"login":"testuser","password":"password123"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusOK,
			},
			storageRes: &storageRes{
				userID: 1,
				err:    nil,
			},
		},
		{
			name: "Невалидный content type",
			request: request{
				body:        `{"login":"testuser","password":"password123"}`,
				contentType: "text/plain",
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Невалидный запрос - нет логина",
			request: request{
				body:        `{"password":"password123"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Невалидный запрос - нет пароля",
			request: request{
				body:        `{"login":"testuser"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Логин уже занят",
			request: request{
				body:        `{"login":"testuser","password":"password123"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusConflict,
			},
			storageRes: &storageRes{
				userID: 0,
				err:    storage.ErrLoginTaken,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(tt.request.body))
			req.Header.Set("Content-Type", tt.request.contentType)
			w := httptest.NewRecorder()

			if tt.storageRes != nil {
				mockStorage.EXPECT().
					CreateUser(gomock.Any(), "testuser", "password123").
					Times(1).
					Return(tt.storageRes.userID, tt.storageRes.err)
			} else {
				mockStorage.EXPECT().CreateUser(gomock.Any(), "testuser", "password123").Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)
		})
	}
}

func TestLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
	}
	type request struct {
		body        string
		contentType string
	}
	type storageRes struct {
		userID int
		err    error
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
	}{
		{
			name: "Успешная аутентификация",
			request: request{
				body:        `{"login":"testuser","password":"password123"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusOK,
			},
			storageRes: &storageRes{
				userID: 1,
				err:    nil,
			},
		},
		{
			name: "Невалидный запрос - Пустые поля",
			request: request{
				body:        `{"login":"","password":""}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Невалидный запрос - Отсутствует логин",
			request: request{
				body:        `{"password":"password123"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Невалидный запрос - Отсутствует пароль",
			request: request{
				body:        `{"login":"testuser"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Некорректный пароль",
			request: request{
				body:        `{"login":"testuser","password":"password"}`,
				contentType: "application/json",
			},
			want: want{
				statusCode: http.StatusUnauthorized,
			},
			storageRes: &storageRes{
				userID: 1,
				err:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(tt.request.body))
			req.Header.Set("Content-Type", tt.request.contentType)
			w := httptest.NewRecorder()

			pwdHash, err := utils.GeneratePasswordHash("password123")
			require.NoError(t, err)
			if tt.storageRes != nil {
				mockStorage.EXPECT().
					GetUserByLogin(gomock.Any(), "testuser").
					Return(&model.User{ID: tt.storageRes.userID, Login: "testuser", PasswordHash: pwdHash}, nil).Times(1)
			} else {
				mockStorage.EXPECT().GetUserByLogin(gomock.Any(), "testuser").Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)
		})
	}
}

func TestCreateNewOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
	}
	type request struct {
		orderNum    string
		contentType string
		isAuth      bool
	}
	type storageRes struct {
		orderID int
		err     error
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
	}{
		{
			name: "Успешный запрос",
			request: request{
				orderNum:    "6485485820226",
				contentType: "text/plain",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusAccepted,
			},
			storageRes: &storageRes{
				orderID: 1,
				err:     nil,
			},
		},
		{
			name: "Номер заказа уже был загружен пользователем",
			request: request{
				orderNum:    "6485485820226",
				contentType: "text/plain",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusOK,
			},
			storageRes: &storageRes{
				orderID: 1,
				err:     storage.ErrOrderNumCreated,
			},
		},
		{
			name: "Номер заказа уже был загружен другим пользователем",
			request: request{
				orderNum:    "6485485820226",
				contentType: "text/plain",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusConflict,
			},
			storageRes: &storageRes{
				orderID: 1,
				err:     storage.ErrOrderNumUsed,
			},
		},
		{
			name: "Неавторизованный запрос",
			request: request{
				orderNum:    "6485485820226",
				contentType: "text/plain",
				isAuth:      false,
			},
			want: want{
				statusCode: http.StatusUnauthorized,
			},
			storageRes: nil,
		},
		{
			name: "Неверный формат запроса",
			request: request{
				orderNum:    "6485485820226",
				contentType: "application/json",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
		},
		{
			name: "Неверный формат номера заказа",
			request: request{
				orderNum:    "123456",
				contentType: "text/plain",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusUnprocessableEntity,
			},
			storageRes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(tt.request.orderNum))
			req.Header.Set("Content-Type", tt.request.contentType)

			if tt.request.isAuth {
				jwtString, err := utils.BuildJWTSting(model.User{ID: 1, Login: "testuser"})
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: middleware.JWTCookieName, Value: jwtString})
			}

			w := httptest.NewRecorder()

			if tt.storageRes != nil {
				mockStorage.EXPECT().
					CreateOrder(gomock.Any(), 1, tt.request.orderNum).
					Return(tt.storageRes.orderID, tt.storageRes.err).
					Times(1)
			} else {
				mockStorage.EXPECT().
					CreateOrder(gomock.Any(), 1, tt.request.orderNum).Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)
		})
	}
}

func TestGetOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
		body       string
	}
	type request struct {
		isAuth bool
	}
	type storageRes struct {
		orders []model.Order
		err    error
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
	}{
		{
			name: "Успешный запрос",
			request: request{
				isAuth: true,
			},
			want: want{
				statusCode: http.StatusOK,
				body: `
					[
						{
									"number": "9278923470",
									"status": "PROCESSED",
									"accrual": 500,
									"uploaded_at": "2020-12-10T15:15:45+03:00"
							},
							{
									"number": "346436439",
									"status": "INVALID",
									"uploaded_at": "2020-12-09T16:09:53+03:00"
							}
					]
				`,
			},
			storageRes: &storageRes{
				err: nil,
				orders: []model.Order{
					{
						ID:        1,
						Number:    "9278923470",
						Status:    model.OrderProcessed,
						Accrual:   500,
						CreatedAt: time.Date(2020, 12, 10, 15, 15, 45, 0, time.Local),
					},
					{
						ID:        2,
						Number:    "346436439",
						Status:    model.OrderInvalid,
						CreatedAt: time.Date(2020, 12, 9, 16, 9, 53, 0, time.Local),
					},
				},
			},
		},
		{
			name: "Нет данных",
			request: request{
				isAuth: true,
			},
			want: want{
				statusCode: http.StatusNoContent,
				body:       "",
			},
			storageRes: &storageRes{
				err:    nil,
				orders: []model.Order{},
			},
		},
		{
			name: "Неавторизованный запрос",
			request: request{
				isAuth: false,
			},
			want: want{
				statusCode: http.StatusUnauthorized,
				body:       "",
			},
			storageRes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)

			if tt.request.isAuth {
				jwtString, err := utils.BuildJWTSting(model.User{ID: 1, Login: "testuser"})
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: middleware.JWTCookieName, Value: jwtString})
			}

			w := httptest.NewRecorder()

			if tt.storageRes != nil {
				mockStorage.EXPECT().
					GetUserOrders(gomock.Any(), 1).
					Return(tt.storageRes.orders, tt.storageRes.err).
					Times(1)
			} else {
				mockStorage.EXPECT().
					GetUserOrders(gomock.Any(), 1).Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)

			if tt.want.body != "" {
				resBody, readErr := io.ReadAll(resp.Body)
				require.NoError(t, readErr)
				assert.JSONEq(t, tt.want.body, string(resBody))
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
		body       string
	}
	type request struct {
		isAuth bool
	}
	type storageRes struct {
		balance *model.Balance
		err     error
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
	}{
		{
			name: "Успешный запрос",
			request: request{
				isAuth: true,
			},
			want: want{
				statusCode: http.StatusOK,
				body: `
					{
						"current": 500.5,
						"withdrawn": 42
					}
				`,
			},
			storageRes: &storageRes{
				err: nil,
				balance: &model.Balance{
					UserID:    1,
					Current:   500.5,
					Withdrawn: 42,
				},
			},
		},
		{
			name: "Неавторизованный запрос",
			request: request{
				isAuth: false,
			},
			want: want{
				statusCode: http.StatusUnauthorized,
				body:       "",
			},
			storageRes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)

			if tt.request.isAuth {
				jwtString, err := utils.BuildJWTSting(model.User{ID: 1, Login: "testuser"})
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: middleware.JWTCookieName, Value: jwtString})
			}

			w := httptest.NewRecorder()

			if tt.storageRes != nil {
				mockStorage.EXPECT().
					GetUserBalance(gomock.Any(), 1).
					Return(tt.storageRes.balance, tt.storageRes.err).
					Times(1)
			} else {
				mockStorage.EXPECT().
					GetUserBalance(gomock.Any(), 1).Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)

			if tt.want.body != "" {
				resBody, readErr := io.ReadAll(resp.Body)
				require.NoError(t, readErr)
				assert.JSONEq(t, tt.want.body, string(resBody))
			}
		})
	}
}

func TestWithdraw(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
	}
	type request struct {
		body        string
		contentType string
		isAuth      bool
	}
	type storageRes struct {
		err error
	}
	type storageReq struct {
		Sum    float64
		Number string
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
		storageReq *storageReq
	}{
		{
			name: "Успешный запрос",
			request: request{
				body:        `{"order":"6485485820226","sum":100}`,
				contentType: "application/json",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusOK,
			},
			storageRes: &storageRes{
				err: nil,
			},
			storageReq: &storageReq{
				Sum:    100,
				Number: "6485485820226",
			},
		},
		{
			name: "На счету недостаточно средств",
			request: request{
				body:        `{"order":"6485485820226","sum":100}`,
				contentType: "application/json",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusPaymentRequired,
			},
			storageRes: &storageRes{
				err: storage.ErrInsufficientFunds,
			},
			storageReq: &storageReq{
				Sum:    100,
				Number: "6485485820226",
			},
		},
		{
			name: "Неверный номер заказа",
			request: request{
				body:        `{"order":"123456","sum":100}`,
				contentType: "application/json",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusUnprocessableEntity,
			},
			storageRes: nil,
			storageReq: nil,
		},
		{
			name: "Некорректная сумма для списания",
			request: request{
				body:        `{"order":"6485485820226","sum":-100}`,
				contentType: "application/json",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusBadRequest,
			},
			storageRes: nil,
			storageReq: nil,
		},
		{
			name: "Неавторизованный запрос",
			request: request{
				body:        `{"order":"6485485820226","sum":-100}`,
				contentType: "application/json",
				isAuth:      false,
			},
			want: want{
				statusCode: http.StatusUnauthorized,
			},
			storageRes: nil,
			storageReq: nil,
		},
		{
			name: "Номер заказа уже был использован",
			request: request{
				body:        `{"order":"6485485820226","sum":100}`,
				contentType: "application/json",
				isAuth:      true,
			},
			want: want{
				statusCode: http.StatusConflict,
			},
			storageRes: &storageRes{
				err: storage.ErrOrderNumUsed,
			},
			storageReq: &storageReq{
				Sum:    100,
				Number: "6485485820226",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(tt.request.body))
			req.Header.Set("Content-Type", tt.request.contentType)

			if tt.request.isAuth {
				jwtString, err := utils.BuildJWTSting(model.User{ID: 1, Login: "testuser"})
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: middleware.JWTCookieName, Value: jwtString})
			}

			w := httptest.NewRecorder()

			if tt.storageRes != nil {
				mockStorage.EXPECT().
					Withdraw(gomock.Any(), 1, tt.storageReq.Sum, tt.storageReq.Number).
					Return(tt.storageRes.err).
					Times(1)
			} else {
				mockStorage.EXPECT().
					Withdraw(gomock.Any(), 1, 1, "1").Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)
		})
	}
}

func TestGetWithdraws(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	router := NewRouter(mockStorage)

	type want struct {
		statusCode int
		body       string
	}
	type request struct {
		isAuth bool
	}
	type storageRes struct {
		withdraws []model.Withdrawn
		err       error
	}

	tests := []struct {
		name       string
		request    request
		want       want
		storageRes *storageRes
	}{
		{
			name: "Успешный запрос",
			request: request{
				isAuth: true,
			},
			want: want{
				statusCode: http.StatusOK,
				body: `
					[
							{
									"order": "2377225624",
									"sum": 500,
									"processed_at": "2020-12-09T16:09:57+03:00"
							}
					]
				`,
			},
			storageRes: &storageRes{
				err: nil,
				withdraws: []model.Withdrawn{
					{
						ID:        1,
						UserID:    1,
						Number:    "2377225624",
						Sum:       500,
						CreatedAt: time.Date(2020, 12, 9, 16, 9, 57, 0, time.Local),
					},
				},
			},
		},
		{
			name: "Нет данных",
			request: request{
				isAuth: true,
			},
			want: want{
				statusCode: http.StatusNoContent,
				body:       "",
			},
			storageRes: &storageRes{
				err:       nil,
				withdraws: []model.Withdrawn{},
			},
		},
		{
			name: "Неавторизованный запрос",
			request: request{
				isAuth: false,
			},
			want: want{
				statusCode: http.StatusUnauthorized,
				body:       "",
			},
			storageRes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)

			if tt.request.isAuth {
				jwtString, err := utils.BuildJWTSting(model.User{ID: 1, Login: "testuser"})
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: middleware.JWTCookieName, Value: jwtString})
			}

			w := httptest.NewRecorder()

			if tt.storageRes != nil {
				mockStorage.EXPECT().
					GetWithdrawals(gomock.Any(), 1).
					Return(tt.storageRes.withdraws, tt.storageRes.err).
					Times(1)
			} else {
				mockStorage.EXPECT().
					GetWithdrawals(gomock.Any(), 1).Times(0)
			}

			router.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.want.statusCode, resp.StatusCode)

			if tt.want.body != "" {
				resBody, readErr := io.ReadAll(resp.Body)
				require.NoError(t, readErr)
				assert.JSONEq(t, tt.want.body, string(resBody))
			}
		})
	}
}
