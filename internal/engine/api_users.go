package engine

import (
	"log"
	"net/http"

	"github.com/mjpitz/myago/auth"
	"github.com/mjpitz/myago/encoding"
)

func (api *API) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := api.users.List(r.Context(), User{})
	if err != nil {
		log.Println(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = encoding.JSON.Encoder(w).Encode(users)
	if err != nil {
		log.Println(err)
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (api *API) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userInfo := auth.Extract(ctx)

	err := encoding.JSON.Encoder(w).Encode(userInfo)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}
