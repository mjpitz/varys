package engine

import (
	"context"
	"errors"
	"net/http"

	"github.com/dgraph-io/badger/v3"

	"github.com/mjpitz/myago"
	"github.com/mjpitz/myago/auth"
)

const userContextKey = myago.ContextKey("varys.user")

func extractUser(ctx context.Context) *User {
	val := ctx.Value(userContextKey)
	if val == nil {
		return nil
	}

	return val.(*User)
}

func withUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey, &user)
}

// Middleware returns an HTTP middleware that manages authenticated users.
func Middleware(handler http.Handler, api *API, authKind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCtx := r.Context()
		var err error

		userInfo := auth.Extract(reqCtx)
		user := User{
			Kind:         authKind,
			ID:           userInfo.Subject,
			Name:         userInfo.Email,
			SiteCounters: map[string]uint32{},
		}

		func() {
			txn := &Txn{txn: api.db.NewTransaction(true)}
			defer txn.CommitOrDiscard(&err)

			ctx := withTxn(reqCtx, txn)

			err = api.users.Get(ctx, user.Kind, user.ID, &user)
			if errors.Is(err, badger.ErrKeyNotFound) {
				err = api.users.Put(ctx, user.Kind, user.ID, user)
			}
		}()

		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		handler.ServeHTTP(w, r.WithContext(withUser(reqCtx, user)))
	})
}
