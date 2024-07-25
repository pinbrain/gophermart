package appctx

import "context"

type ctxKey string

type CtxUser struct {
	ID    int
	Login string
}

const (
	userCtxKey ctxKey = "user"
)

func CtxWithUser(ctx context.Context, user *CtxUser) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

func GetCtxUser(ctx context.Context) *CtxUser {
	val := ctx.Value(userCtxKey)
	user, ok := val.(*CtxUser)
	if !ok {
		return nil
	}
	return user
}
