package engine

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/myago/pass"
)

func (api *API) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)

	service := Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	err := api.services.Get(ctx, service.Kind, service.Name, &service)
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	// enumerate users for service
	http.Error(w, "unimplemented", http.StatusNotFound)
}

type ListCredentialsResponse struct{}

func (api *API) GetCurrentUserCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := extractUser(ctx)

	vars := mux.Vars(r)

	service := Service{
		Kind: vars["kind"],
		Name: vars["name"],
	}

	err := api.services.Get(ctx, service.Kind, service.Name, &service)
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	username, password, err := Derive(api.root, service, user)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	response := GetCurrentUserCredentials{
		Address: service.Address,
		Credentials: Credentials{
			Username: string(username),
			Password: string(password),
		},
	}

	err = encoding.JSON.Encoder(w).Encode(response)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type GetCurrentUserCredentials struct {
	Address     string
	Credentials Credentials
}

// Derive provides a convenience function for producing a username and password for a site given the site config.
func Derive(root string, site Service, user *User) (username, password []byte, err error) {
	counter := user.SiteCounters[site.Kind+"/"+site.Name]

	username, err = derive(root, pass.Identification, site, user.Name, counter)
	if err != nil {
		return nil, nil, err
	}

	password, err = derive(root, pass.Authentication, site, string(username), counter)
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
