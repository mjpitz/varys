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
	"strings"

	"github.com/dgraph-io/badger/v3"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/myago/pass"
	"github.com/mjpitz/myago/zaputil"
)

func (api *API) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	q := r.URL.Query()
	vars := mux.Vars(r)

	service := Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	if service.Kind == "" || service.Name == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	err := api.services.Get(ctx, service.Kind, service.Name, &service)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		http.Error(w, "", http.StatusNotFound)
		return
	case err != nil:
		log.Error("failed to get service", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	permissions := []Permission{ReadPermission, WritePermission, UpdatePermission, DeletePermission, AdminPermission}
	if param := q.Get("permissions"); len(param) > 0 {
		perms := strings.Split(param, ",")
		permissions = make([]Permission, 0, len(perms))

		for _, perm := range perms {
			if p := Permission(perm); p.String() != "" && p != SystemPermission {
				permissions = append(permissions, p)
			}
		}
	}

	credentials := make([]UserCredential, 0)
	userKeys := make(map[string]int)

	for _, perm := range permissions {
		roles := []string{
			fmt.Sprintf("%s:%s:%s", perm, service.Kind, service.Name),
			fmt.Sprintf("%s:%s", perm, service.Kind),
		}

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
					userKeys[user] = len(credentials)
					credentials = append(credentials, UserCredential{})
				}

				credentials[userKeys[user]].Permission = append(credentials[userKeys[user]].Permission, perm)
			}
		}
	}

	txn := &Txn{api.db.NewTransaction(false)}
	defer txn.CommitOrDiscard(&err)

	ctx = withTxn(ctx, txn)

	for key, idx := range userKeys {
		parts := strings.Split(key, "/")
		user := &User{}

		err = api.users.Get(ctx, parts[0], parts[1], user)
		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
			continue
		case err != nil:
			log.Error("failed to get user for key", zap.Error(err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		username, password, err := Derive(api.root, service, user)
		if err != nil {
			log.Error("failed to derive credentials", zap.Error(err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		credentials[idx].Credentials.Username = string(username)
		credentials[idx].Credentials.Password = string(password)
	}

	err = encoding.JSON.Encoder(w).Encode(credentials)
	if err != nil {
		log.Error("failed to marshal json", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type UserCredential struct {
	Permission  []Permission `json:"permissions"`
	Credentials Credentials  `json:"credentials"`
}

func (api *API) GetServiceCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)
	user := extractUser(ctx)

	vars := mux.Vars(r)

	service := Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	if service.Kind == "" || service.Name == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	err := api.services.Get(ctx, service.Kind, service.Name, &service)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		http.Error(w, "", http.StatusNotFound)
		return
	case err != nil:
		log.Error("failed to get service", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	match := "(" + strings.Join([]string{
		ReadPermission.String(),
		WritePermission.String(),
		UpdatePermission.String(),
		DeletePermission.String(),
		AdminPermission.String(),
	}, ")|(") + ")"

	allowed, err := api.enforcer.Enforce(user.K(), service.K(), match)
	if err != nil {
		log.Error("failed to enforce credentials", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	} else if !allowed {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	username, password, err := Derive(api.root, service, user)
	if err != nil {
		log.Error("failed to derive credentials", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	response := ServiceCredentials{
		Address: service.Address,
		Credentials: Credentials{
			Username: string(username),
			Password: string(password),
		},
	}

	err = encoding.JSON.Encoder(w).Encode(response)
	if err != nil {
		log.Error("failed to marshal json", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type ServiceCredentials struct {
	Address     string      `json:"address"`
	Credentials Credentials `json:"credentials"`
}

// Derive provides a convenience function for producing a username and password for a site given the site config.
func Derive(root string, service Service, user *User) (username, password []byte, err error) {
	counter := user.SiteCounters[service.K()]

	username, err = derive(root, pass.Identification, service, user.Name, counter)
	if err != nil {
		return nil, nil, err
	}

	password, err = derive(root, pass.Authentication, service, string(username), counter)
	if err != nil {
		return nil, nil, err
	}

	return username, password, nil
}

func derive(root string, scope pass.Scope, site Service, name string, counter uint32) ([]byte, error) {
	key, err := pass.Identity(pass.Authentication, site.Key, root)
	if err != nil {
		return nil, err
	}

	identity, err := pass.Identity(scope, key, name)
	if err != nil {
		return nil, err
	}

	siteKey := pass.SiteKey(scope, identity, site.Address, counter)

	switch scope {
	case pass.Identification:
		return pass.SitePassword(siteKey, site.Templates.UserTemplate), nil
	default:
		return pass.SitePassword(siteKey, site.Templates.PasswordTemplate), nil
	}
}
