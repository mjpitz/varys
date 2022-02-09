package engine

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/myago/pass"
)

func (api *API) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	permissions := []Permission{ReadPermission, WritePermission, DeletePermission, AdminPermission}
	if param := q.Get("permissions"); len(param) > 0 {
		perms := strings.Split(param, ",")
		permissions = make([]Permission, 0, len(perms))

		for _, perm := range perms {
			if p := Permission(perm); p.String() != "" && p != SystemPermission {
				permissions = append(permissions, p)
			}
		}
	}

	txn := &Txn{api.db.NewTransaction(false)}
	defer txn.CommitOrDiscard(&err)

	ctx = withTxn(ctx, txn)

	credentials := make([]*UserCredential, 0)

	// TODO: resolve users who have access to this resource

	err = encoding.JSON.Encoder(w).Encode(credentials)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}

type UserCredential struct {
	Permission  []Permission `json:"permissions"`
	Credentials Credentials  `json:"credentials"`
}

func (api *API) GetCurrentUserCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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
