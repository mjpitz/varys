// Copyright (C) 2022  Mya Pitzeruse
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

package engine

import (
	"context"
	"errors"
	"net/http"

	"github.com/dgraph-io/badger/v3"
	"go.uber.org/zap"

	"github.com/mjpitz/myago"
	"github.com/mjpitz/myago/auth"
	"github.com/mjpitz/myago/zaputil"
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
		log := zaputil.Extract(reqCtx)

		var err error

		userInfo := auth.Extract(reqCtx)
		user := User{
			Kind:         authKind,
			ID:           userInfo.Subject,
			Name:         userInfo.Profile,
			SiteCounters: map[string]uint32{},
		}

		func() {
			txn := &Txn{txn: api.db.NewTransaction(true)}
			defer txn.CommitOrDiscard(&err)

			ctx := withTxn(reqCtx, txn)

			err = api.users.Get(ctx, user.Kind, user.ID, &user)
			if errors.Is(err, badger.ErrKeyNotFound) {
				err = api.users.Put(ctx, user.Kind, user.ID, user)
				if err != nil {
					log.Error("failed to create user", zap.Error(err))
					return
				}

				_, err = api.enforcer.AddRolesForUser(user.K(), []string{
					"read:varys",
				})
				if err != nil {
					log.Error("failed to add default roles for user", zap.Error(err))
				}
			}
		}()

		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		allowed, err := api.enforcer.Enforce(user.K(), r.URL.Path, r.Method)
		if err != nil {
			log.Error("failed to enforce access", zap.Error(err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		} else if !allowed {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r.WithContext(withUser(reqCtx, user)))
	})
}
