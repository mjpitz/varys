package client

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"

	"github.com/mjpitz/myago"
	"github.com/mjpitz/myago/auth"
	basicauth "github.com/mjpitz/myago/auth/basic"
	"github.com/mjpitz/myago/encoding"
	"github.com/mjpitz/varys/internal/engine"
)

var (
	contextKey = myago.ContextKey("varys.client")

	DefaultConfig = Config{
		BaseURL: "http://localhost:3456",
	}
)

func Extract(ctx context.Context) *API {
	val := ctx.Value(contextKey)
	if val == nil {
		return nil
	}

	return val.(*API)
}

func WithContext(ctx context.Context, api *API) context.Context {
	return context.WithValue(ctx, contextKey, api)
}

type Config struct {
	BaseURL string                 `json:"base_url" usage:"the base url that points to a varys instance"`
	Basic   basicauth.ClientConfig `json:"basic"`
}

func NewAPI(cfg Config) (*API, error) {
	token, err := cfg.Basic.Token()
	if err != nil {
		return nil, err
	}

	return &API{
		baseURL: cfg.BaseURL,
		token:   token,
	}, nil
}

type API struct {
	baseURL string
	token   *oauth2.Token
}

func (api *API) Do(ctx context.Context, method, path string, req interface{}, res interface{}) error {
	body := bytes.NewBuffer(nil)

	if req != nil {
		err := encoding.JSON.Encoder(body).Encode(req)
		if err != nil {
			return err
		}
	}

	r, err := http.NewRequestWithContext(ctx, method, api.baseURL+path, body)
	if err != nil {
		return err
	}

	if api.token != nil {
		api.token.SetAuthHeader(r)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 400 {
		return fmt.Errorf(resp.Status)
	}

	if res != nil {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		err = encoding.JSON.Decoder(bytes.NewBuffer(data)).Decode(res)
		if err != nil {
			return err
		}
	}

	return nil
}

func (api *API) Services() *Services {
	return &Services{api}
}

func (api *API) Users() *Users {
	return &Users{api}
}

type Services struct {
	api *API
}

func (s *Services) Grants() *Grants {
	return &Grants{s.api}
}

func (s *Services) List(ctx context.Context) ([]engine.Service, error) {
	services := make([]engine.Service, 0)
	err := s.api.Do(ctx, http.MethodGet, "/api/v1/services", nil, &services)

	return services, err
}

func (s *Services) Get(ctx context.Context, kind, name string) (engine.Service, error) {
	path := fmt.Sprintf("/api/v1/services/%s/%s", url.PathEscape(kind), url.PathEscape(name))

	service := engine.Service{}
	err := s.api.Do(ctx, http.MethodGet, path, nil, &service)

	return service, err
}

func (s *Services) Credentials(ctx context.Context, kind, name string) (engine.ServiceCredentials, error) {
	path := fmt.Sprintf("/api/v1/services/%s/%s/credentials", url.PathEscape(kind), url.PathEscape(name))

	credentials := engine.ServiceCredentials{}
	err := s.api.Do(ctx, http.MethodGet, path, nil, &credentials)

	return credentials, err
}

func (s *Services) Create(ctx context.Context, req engine.CreateServiceRequest) error {
	return s.api.Do(ctx, http.MethodPost, "/api/v1/services", req, nil)
}

func (s *Services) Update(ctx context.Context, kind, name string, req engine.UpdateServiceRequest) error {
	path := fmt.Sprintf("/api/v1/services/%s/%s", url.PathEscape(kind), url.PathEscape(name))

	return s.api.Do(ctx, http.MethodPut, path, req, nil)
}

func (s *Services) Delete(ctx context.Context, kind, name string) error {
	path := fmt.Sprintf("/api/v1/services/%s/%s", url.PathEscape(kind), url.PathEscape(name))

	return s.api.Do(ctx, http.MethodDelete, path, nil, nil)
}

type Grants struct {
	api *API
}

func (a *Grants) List(ctx context.Context, kind, name string) ([]engine.UserGrant, error) {
	path := fmt.Sprintf("/api/v1/services/%s/%s/grants", url.PathEscape(kind), url.PathEscape(name))

	resp := engine.ListGrantsResponse{}
	err := a.api.Do(ctx, http.MethodGet, path, nil, &resp)

	return resp.Grants, err
}

func (a *Grants) Update(ctx context.Context, kind, name string, grant engine.UserGrant) error {
	path := fmt.Sprintf("/api/v1/services/%s/%s/grants", url.PathEscape(kind), url.PathEscape(name))

	return a.api.Do(ctx, http.MethodPut, path, grant, nil)
}

func (a *Grants) Delete(ctx context.Context, kind, name string, grant engine.UserGrant) error {
	path := fmt.Sprintf("/api/v1/services/%s/%s/grants", url.PathEscape(kind), url.PathEscape(name))

	return a.api.Do(ctx, http.MethodDelete, path, grant, nil)
}

type Users struct {
	api *API
}

func (u *Users) List(ctx context.Context) ([]engine.User, error) {
	users := make([]engine.User, 0)
	err := u.api.Do(ctx, http.MethodGet, "/api/v1/users", nil, &users)

	return users, err
}

func (u *Users) Current(ctx context.Context) (auth.UserInfo, error) {
	info := auth.UserInfo{}
	err := u.api.Do(ctx, http.MethodGet, "/api/v1/users/self", nil, &info)

	return info, err
}
