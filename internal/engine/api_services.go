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
	"crypto/rand"
	"errors"
	"net/http"

	"github.com/dgraph-io/badger/v3"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/myago/pass"
	"github.com/mjpitz/myago/zaputil"
)

func (api *API) ListServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	services, err := api.services.List(ctx, Service{})
	if err != nil {
		log.Error("failed to list services", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = encoding.JSON.Encoder(w).Encode(services)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (api *API) CreateService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)
	user := extractUser(ctx)

	req := CreateServiceRequest{}
	err := encoding.JSON.Decoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	service := Service{
		Kind:    req.Kind,
		Name:    req.Name,
		Address: req.Address,
		Key:     make([]byte, 32),
		Templates: ServiceTemplates{
			UserTemplate:     pass.Basic,
			PasswordTemplate: pass.MaximumSecurity,
		},
	}

	if req.Templates.UserTemplate != "" {
		service.Templates.UserTemplate = pass.TemplateClass(req.Templates.UserTemplate)
	}

	if req.Templates.PasswordTemplate != "" {
		service.Templates.PasswordTemplate = pass.TemplateClass(req.Templates.PasswordTemplate)
	}

	if service.Kind == "" || service.Name == "" || service.Address == "" ||
		service.Templates.UserTemplate == "" || service.Templates.PasswordTemplate == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	policy, err := renderServicePolicy(policyTemplate{
		Service: service,
		Creator: *user,
	})

	if err != nil {
		log.Error("failed to render service policy", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if _, err = rand.Read(service.Key); err != nil {
		log.Error("failed to generate service key", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	func() {
		txn := &Txn{api.db.NewTransaction(true)}
		defer txn.CommitOrDiscard(&err)

		ctx := withTxn(ctx, txn)

		err = api.services.Get(ctx, service.Kind, service.Name, &service)
		if err == nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		err = api.services.Put(ctx, service.Kind, service.Name, service)
		if err != nil {
			log.Error("failed to create service", zap.Error(err))
			return
		}

		err = EnsurePolicy(api.enforcer, policy)
		if err != nil {
			log.Error("failed to ensure policy for service", zap.Error(err))
		}
	}()

	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

type CreateServiceRequest struct {
	Kind    string `json:"kind" hidden:"true"`
	Name    string `json:"name" hidden:"true"`
	Address string `json:"address" usage:"the address clients should connect to" required:"true"`
	Templates
}

func (api *API) getService(r *http.Request) (*Service, int) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	vars := mux.Vars(r)

	service := &Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	if service.Kind == "" || service.Name == "" {
		return nil, http.StatusBadRequest
	}

	err := api.services.Get(ctx, service.Kind, service.Name, service)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		return nil, http.StatusNotFound
	case err != nil:
		log.Error("failed to get service", zap.Error(err))
		return nil, http.StatusInternalServerError
	}

	return service, 0
}

func (api *API) GetService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	service, code := api.getService(r)
	if code > 0 {
		http.Error(w, "", code)
		return
	}

	err := encoding.JSON.Encoder(w).Encode(service)
	if err != nil {
		log.Error("", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (api *API) UpdateService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	req := UpdateServiceRequest{}
	err := encoding.JSON.Decoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	txn := &Txn{api.db.NewTransaction(true)}
	defer txn.CommitOrDiscard(&err)

	ctx = withTxn(ctx, txn)

	service, code := api.getService(r.WithContext(ctx))
	if code > 0 {
		http.Error(w, "", code)
		return
	}

	if req.RotateKey {
		if _, err = rand.Read(service.Key); err != nil {
			log.Error("failed to regenerate service key", zap.Error(err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	if req.Address != "" {
		service.Address = req.Address
	}

	if req.Templates.UserTemplate != "" {
		service.Templates.UserTemplate = pass.TemplateClass(req.Templates.UserTemplate)
	}

	if req.Templates.PasswordTemplate != "" {
		service.Templates.PasswordTemplate = pass.TemplateClass(req.Templates.PasswordTemplate)
	}

	err = api.services.Put(ctx, service.Kind, service.Name, service)
	if err != nil {
		log.Error("failed to update service", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type UpdateServiceRequest struct {
	RotateKey bool   `json:"rotate_key" usage:"set to rotate the key used to derive passwords for this service"`
	Address   string `json:"address" usage:"the new address clients should connect to"`
	Templates
}

func (api *API) DeleteService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zaputil.Extract(ctx)

	vars := mux.Vars(r)

	service := &Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	if service.Kind == "" || service.Name == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	err := api.services.Delete(ctx, service.Kind, service.Name)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		http.Error(w, "", http.StatusNotFound)
	case err != nil:
		log.Error("failed to delete service", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}
