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
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/mjpitz/myago/auth"
	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/myago/zaputil"
)

func (api *API) getUsersForRole(role string) ([]string, error) {
	users, err := api.enforcer.GetUsersForRole(role)
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0)
	for _, user := range users {
		if !strings.HasPrefix(user, "/_user/") {
			continue
		}

		user = strings.TrimPrefix(user, "/_user/")
		filtered = append(filtered, user)
	}

	return filtered, nil
}

func (api *API) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	users, err := api.users.List(ctx, User{})
	if err != nil {
		log.Error("failed to list users", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = encoding.JSON.Encoder(w).Encode(users)
	if err != nil {
		log.Error("failed to marshal json", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (api *API) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)
	userInfo := auth.Extract(ctx)

	err := encoding.JSON.Encoder(w).Encode(userInfo)
	if err != nil {
		log.Error("failed to marshal json", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}
