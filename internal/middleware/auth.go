package middleware

import (
	"net/http"

	"github.com/pinbrain/gophermart/internal/appctx"
	"github.com/pinbrain/gophermart/internal/utils"
)

const (
	JWTCookieName = "gophermart_jwt"
)

func newCookie(name, value string) *http.Cookie {
	cookie := http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
	}
	return &cookie
}

func SetJWTCookie(w http.ResponseWriter, value string) {
	cookie := newCookie(JWTCookieName, value)
	http.SetCookie(w, cookie)
}

func DeleteJWTCookie(w http.ResponseWriter) {
	cookie := newCookie(JWTCookieName, "")
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}

func RequireUser(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, err := r.Cookie(JWTCookieName)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		jwtClaims, err := utils.GetJWTClaims(jwtCookie.Value)
		if err != nil {
			DeleteJWTCookie(w)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := r.Context()
		ctx = appctx.CtxWithUser(ctx, &appctx.CtxUser{
			ID:    jwtClaims.UserID,
			Login: jwtClaims.Login,
		})
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}
