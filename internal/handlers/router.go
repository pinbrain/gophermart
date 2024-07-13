package handlers

import (
	"github.com/go-chi/chi/v5"
	"github.com/pinbrain/gophermart/internal/middleware"
)

func NewRouter(storage Storage) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.HTTPRequestLogger)

	userHandler := newUserHandler(storage)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", userHandler.RegisterUser)
		r.Post("/login", userHandler.Login)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireUser)
			r.Post("/orders", userHandler.CreateNewOrder)
			r.Get("/orders", userHandler.GetOrders)
			r.Get("/balance", userHandler.GetBalance)
			r.Post("/balance/withdraw", userHandler.Withdraw)
			r.Get("/withdrawals", userHandler.GetWithdraws)
		})
	})

	return r
}
