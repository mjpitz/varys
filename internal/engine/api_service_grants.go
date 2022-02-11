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
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/dgraph-io/badger/v3"
	"go.uber.org/zap"

	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/myago/zaputil"
)

func (api *API) ListGrants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	var err error

	service, code := api.getService(r)
	if code > 0 {
		http.Error(w, "", code)
		return
	}

	resp := ListGrantsResponse{}
	userKeys := make(map[string]int)

	for _, perm := range PermissionValues {
		roles := []string{
			fmt.Sprintf("%s:%s:%s", perm, service.Kind, service.Name),
			fmt.Sprintf("%s:%s", perm, service.Kind),
		}
		resp.Roles = append(resp.Roles, roles[0])

		for _, role := range roles {
			users, err := api.getUsersForRole(role)
			if err != nil {
				log.Error("failed to get users for role", zap.Error(err))
				http.Error(w, "", http.StatusInternalServerError)
				return
			}

			for _, user := range users {
				_, ok := userKeys[user]
				if !ok {
					userKeys[user] = len(resp.Grants)
					resp.Grants = append(resp.Grants, UserGrant{})
				}

				resp.Grants[userKeys[user]].Roles = append(resp.Grants[userKeys[user]].Roles, role)
			}
		}
	}

	txn := &Txn{api.db.NewTransaction(false)}
	defer txn.CommitOrDiscard(&err)

	ctx = withTxn(ctx, txn)

	prune := make([]int, 0)
	for key, idx := range userKeys {
		parts := strings.Split(key, "/")

		err := api.users.Get(ctx, parts[0], parts[1], &resp.Grants[idx].User)
		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
			prune = append(prune, idx)
			continue
		case err != nil:
			log.Error("failed to get user for key", zap.Error(err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	sort.Ints(prune)

	p := len(prune)
	for i := 0; i < p; i++ {
		idx := prune[p-i-1]

		resp.Grants = append(resp.Grants[:idx], append([]UserGrant{}, resp.Grants[idx+1:]...)...)
	}

	err = encoding.JSON.Encoder(w).Encode(resp)
	if err != nil {
		log.Error("failed to marshal json", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type UserGrant struct {
	User  User     `json:"user"`
	Roles []string `json:"roles"`
}

type ListGrantsResponse struct {
	Roles  []string    `json:"assignable_roles"`
	Grants []UserGrant `json:"grants"`
}

func (api *API) PutGrant(w http.ResponseWriter, r *http.Request) {
	req := UserGrant{}
	err := encoding.JSON.Decoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	log := zaputil.Extract(ctx)

	service, code := api.getService(r)
	if code > 0 {
		http.Error(w, "", code)
		return
	}

	suffix := fmt.Sprintf("%s:%s", service.Kind, service.Name)

	roles := make(map[string]bool)
	for _, perm := range PermissionValues {
		roles[perm.String()+":"+suffix] = true
	}

	added := make([]string, 0)
	for _, role := range req.Roles {
		if roles[role] {
			added = append(added, role)
		}
	}

	userKey := req.User.K()

	_, err = api.enforcer.AddRolesForUser(userKey, added)
	if err != nil {
		log.Error("failed to add roles for user", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (api *API) DeleteGrant(w http.ResponseWriter, r *http.Request) {
	req := UserGrant{}
	err := encoding.JSON.Decoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	log := zaputil.Extract(ctx)

	service, code := api.getService(r)
	if code > 0 {
		http.Error(w, "", code)
		return
	}

	userKey := req.User.K()

	suffix := fmt.Sprintf("%s:%s", service.Kind, service.Name)

	roles := make(map[string]bool)
	for _, perm := range PermissionValues {
		roles[perm.String()+":"+suffix] = true
	}

	for _, role := range req.Roles {
		if roles[role] {
			_, err := api.enforcer.DeleteRoleForUser(userKey, role)
			if err != nil {
				log.Error("failed to delete role for user", zap.Error(err))
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		}
	}
}
