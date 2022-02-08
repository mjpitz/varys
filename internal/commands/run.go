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

package commands

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/mjpitz/myago/auth"
	basicauth "github.com/mjpitz/myago/auth/basic"
	httpauth "github.com/mjpitz/myago/auth/http"
	"github.com/mjpitz/myago/flagset"
	"github.com/mjpitz/myago/headers"
	"github.com/mjpitz/myago/livetls"
	"github.com/mjpitz/myago/zaputil"
	"github.com/mjpitz/varys/internal/engine"
)

type EncryptionConfig struct {
	Key         string        `json:"key"          usage:"specify the root encryption key used to encrypt the database"`
	KeyDuration time.Duration `json:"key_duration" usage:"how long a derived encryption key is good for" default:"120h"`
}

type DatabaseConfig struct {
	Path       string           `json:"path"       usage:"configure the path to the database" default:"db.badger"`
	Encryption EncryptionConfig `json:"encryption" `
}

type CredentialConfig struct {
	RootKey string `json:"root_key" usage:"specify the root key used to derive credentials from"`
}

type RunConfig struct {
	BindAddress string           `json:"bind_address" usage:"specify the address to bind to" default:"localhost:3456"`
	TLS         livetls.Config   `json:"tls"`
	Database    DatabaseConfig   `json:"database"`
	Credential  CredentialConfig `json:"credential"`

	auth.Config
	Basic basicauth.Config `json:"basic"`
}

type admin struct {
	log *zap.Logger
}

func (a *admin) Lookup(req basicauth.LookupRequest) (resp basicauth.LookupResponse, err error) {
	if req.User != "badadmin" {
		err = auth.ErrUnauthorized
	} else if len(req.Token) > 0 {
		err = basicauth.ErrBadRequest
	}

	if err == nil {
		resp = basicauth.LookupResponse{
			UserID:   req.User,
			User:     req.User,
			Email:    req.User,
			Password: "badadmin",
		}
	}

	return
}

var (
	runConfig = &RunConfig{}

	Run = &cli.Command{
		Name:      "run",
		Usage:     "Runs the varys server process",
		UsageText: "varys run [OPTIONS]",
		Flags:     flagset.ExtractPrefix("varys", runConfig),
		Action: func(ctx *cli.Context) error {
			log := zaputil.Extract(ctx.Context)

			tlsConfig, err := livetls.New(ctx.Context, runConfig.TLS)
			if err != nil {
				return err
			}

			var authFn auth.HandlerFunc

			switch runConfig.AuthType {
			case "basic":
				log.Info("configuring basic auth")
				authFn, err = basicauth.Handler(ctx.Context, runConfig.Basic)
				if err != nil {
					return err
				}
			case "oidc":
				log.Info("configuring oidc auth")
				return fmt.Errorf("oidc not yet supported")
			default:
				log.Info("configuring default auth")
				runConfig.AuthType = "default"
				authFn = basicauth.Basic(&admin{log})
			}

			encryptionKey := sha256.Sum256([]byte(runConfig.Database.Encryption.Key))

			opts := badger.DefaultOptions(runConfig.Database.Path)
			opts.Logger = zaputil.Badger(log)
			opts.EncryptionKey = encryptionKey[:]
			opts.EncryptionKeyRotationDuration = runConfig.Database.Encryption.KeyDuration
			opts.IndexCacheSize = 128 << 20 // 128 MiB

			db, err := badger.Open(opts)
			if err != nil {
				return err
			}
			defer db.Close()

			router := mux.NewRouter()
			router.StrictSlash(true)
			router.SkipClean(true)
			router.Use(mux.CORSMethodMiddleware(router))

			api := engine.NewAPI(db, runConfig.Credential.RootKey)

			apiRouter := router.PathPrefix("/api/").Subrouter()
			apiRouter.Use(func(handler http.Handler) http.Handler {
				// handler needs to be in reverse order since it works using delegation
				handler = engine.Middleware(handler, api, runConfig.AuthType)
				handler = httpauth.Handler(handler, authFn, auth.Required())
				handler = headers.HTTP(handler)

				return handler
			})

			credentials := apiRouter.PathPrefix("/v1/credentials").Subrouter()
			credentials.HandleFunc("/{kind}/{name}", api.ListCredentials).Methods(http.MethodGet)
			credentials.HandleFunc("/{kind}/{name}/self", api.GetCurrentUserCredentials).Methods(http.MethodGet)

			services := apiRouter.PathPrefix("/v1/services").Subrouter()
			services.HandleFunc("", api.ListServices).Methods(http.MethodGet)
			services.HandleFunc("", api.CreateService).Methods(http.MethodPost)
			services.HandleFunc("/{kind}/{name}", api.GetService).Methods(http.MethodGet)
			services.HandleFunc("/{kind}/{name}", api.UpdateService).Methods(http.MethodPut)
			services.HandleFunc("/{kind}/{name}", api.DeleteService).Methods(http.MethodDelete)

			users := apiRouter.PathPrefix("/v1/users").Subrouter()
			users.HandleFunc("", api.ListUsers).Methods(http.MethodGet)
			users.HandleFunc("/self", api.GetCurrentUser).Methods(http.MethodGet)

			group, done := errgroup.WithContext(ctx.Context)
			group.Go(func() error {
				listener, err := net.Listen("tcp", runConfig.BindAddress)
				if err != nil {
					return err
				}

				if tlsConfig != nil {
					listener = tls.NewListener(listener, tlsConfig)
				}

				svr := &http.Server{
					Handler: router,
					BaseContext: func(listener net.Listener) context.Context {
						return ctx.Context
					},
				}

				return svr.Serve(listener)
			})

			log.Info("starting", zap.String("address", runConfig.BindAddress))
			if log.Core().Enabled(zapcore.DebugLevel) {
				_ = router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
					path, _ := route.GetPathTemplate()
					methods, _ := route.GetMethods()

					if len(methods) > 0 {
						log.Debug("route", zap.String("path", path), zap.Strings("methods", methods))
					}

					return nil
				})
			}

			select {
			case <-done.Done():
				return group.Wait()
			case <-ctx.Done():
			}

			return nil
		},
		HideHelpCommand: true,
	}
)
