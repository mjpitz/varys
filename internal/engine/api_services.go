package engine

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strconv"

	"github.com/dgraph-io/badger/v3"
	"github.com/gorilla/mux"

	"github.com/mjpitz/myago/encoding"
)

func (api *API) ListServices(w http.ResponseWriter, r *http.Request) {
	services, err := api.services.List(r.Context(), Service{})

	err = encoding.JSON.Encoder(w).Encode(services)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (api *API) CreateService(w http.ResponseWriter, r *http.Request) {
	req := CreateServiceRequest{}
	err := encoding.JSON.Decoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	service := &Service{
		Kind:      req.Kind,
		Name:      req.Name,
		Address:   req.Address,
		Key:       make([]byte, 32),
		Templates: req.Templates,
	}

	if service.Kind == "" || service.Name == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if _, err = rand.Read(service.Key); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	func() {
		txn := &Txn{api.db.NewTransaction(true)}
		defer txn.CommitOrDiscard(&err)

		ctx := withTxn(r.Context(), txn)

		err = api.services.Get(ctx, service.Kind, service.Name, service)
		if err == nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		err = api.services.Put(ctx, service.Kind, service.Name, service)
	}()

	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type CreateServiceRequest struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Templates Templates `json:"templates"`
}

func (api *API) GetService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	service := &Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	err := api.services.Get(r.Context(), service.Kind, service.Name, service)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		http.Error(w, "", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = encoding.JSON.Encoder(w).Encode(service)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (api *API) UpdateService(w http.ResponseWriter, r *http.Request) {
	req := UpdateServiceRequest{}
	err := encoding.JSON.Decoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	query := r.URL.Query()

	rotateKey, _ := strconv.ParseBool(query.Get("rotate_key"))

	service := &Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	txn := &Txn{api.db.NewTransaction(true)}
	defer txn.CommitOrDiscard(&err)

	ctx := withTxn(r.Context(), txn)

	err = api.services.Get(ctx, service.Kind, service.Name, service)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		http.Error(w, "", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if rotateKey {
		if _, err = rand.Read(service.Key); err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	if req.Address != nil {
		service.Address = *req.Address
	}

	if req.Address != nil {
		service.Templates = *req.Templates
	}

	err = api.services.Put(ctx, service.Kind, service.Name, service)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type UpdateServiceRequest struct {
	Address   *string    `json:"address"`
	Templates *Templates `json:"templates"`
}

func (api *API) DeleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	service := &Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	err := api.services.Delete(r.Context(), service.Kind, service.Name)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		http.Error(w, "", http.StatusNotFound)
	case err != nil:
		http.Error(w, "", http.StatusInternalServerError)
	}
}
