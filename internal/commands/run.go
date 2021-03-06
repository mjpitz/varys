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

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
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
	KeyDuration time.Duration `json:"key_duration" usage:"how long a derived encryption key is good for" default:"120h" hidden:"true"`
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

var (
	runConfig = &RunConfig{
		Config: auth.Config{
			AuthType: "basic",
		},
		Basic: basicauth.Config{
			StaticUsername: "badadmin",
			StaticPassword: "badadmin",
			StaticGroups:   cli.NewStringSlice(),
		},
	}

	Run = &cli.Command{
		Name:      "run",
		Usage:     "Runs the varys server process.",
		Flags:     flagset.ExtractPrefix("varys", runConfig),
		ArgsUsage: " ",
		Action: func(ctx *cli.Context) error {
			log := zaputil.Extract(ctx.Context)

			if v := runConfig.Basic.StaticGroups.Value(); len(v) == 0 {
				_ = runConfig.Basic.StaticGroups.Set("admin:varys")
			}

			tlsConfig, err := livetls.New(ctx.Context, runConfig.TLS)
			if err != nil {
				return err
			}

			log.Info("configuring auth", zap.String("kind", runConfig.AuthType))
			var authFn auth.HandlerFunc

			switch runConfig.AuthType {
			case "basic":
				authFn, err = basicauth.Handler(ctx.Context, runConfig.Basic)
				if err != nil {
					return err
				}
			case "oidc":
				return fmt.Errorf("oidc not yet supported")
			default:
				return fmt.Errorf("unsupported auth type: %s", runConfig.AuthType)
			}

			log.Info("opening database")
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

			adapter := engine.NewCasbinAdapter(db)
			model, err := model.NewModelFromString(engine.Model)
			if err != nil {
				return err
			}

			enforcer, _ := casbin.NewEnforcer()
			enforcer.SetModel(model)
			enforcer.SetAdapter(adapter)

			enforcer.EnableAutoSave(true)
			enforcer.EnableAutoBuildRoleLinks(true)

			err = enforcer.LoadPolicy()
			if err != nil {
				return err
			}

			err = engine.EnsurePolicy(enforcer, engine.DefaultPolicy)
			if err != nil {
				return err
			}

			log.Info("setting up api")
			api := engine.NewAPI(db, enforcer, runConfig.Credential.RootKey)

			router := mux.NewRouter()
			router.StrictSlash(true)
			router.SkipClean(true)
			router.Use(mux.CORSMethodMiddleware(router))

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

			services := apiRouter.PathPrefix("/v1/services").Subrouter()
			services.HandleFunc("", api.ListServices).Methods(http.MethodGet)
			services.HandleFunc("", api.CreateService).Methods(http.MethodPost)
			services.HandleFunc("/{kind}/{name}", api.GetService).Methods(http.MethodGet)
			services.HandleFunc("/{kind}/{name}", api.UpdateService).Methods(http.MethodPut)
			services.HandleFunc("/{kind}/{name}", api.DeleteService).Methods(http.MethodDelete)
			services.HandleFunc("/{kind}/{name}/credentials", api.GetServiceCredentials).Methods(http.MethodGet)
			services.HandleFunc("/{kind}/{name}/grants", api.ListGrants).Methods(http.MethodGet)
			services.HandleFunc("/{kind}/{name}/grants", api.PutGrant).Methods(http.MethodPut)
			services.HandleFunc("/{kind}/{name}/grants", api.DeleteGrant).Methods(http.MethodDelete)

			users := apiRouter.PathPrefix("/v1/users").Subrouter()
			users.HandleFunc("", api.ListUsers).Methods(http.MethodGet)
			users.HandleFunc("/self", api.GetCurrentUser).Methods(http.MethodGet)
			users.HandleFunc("/self", api.UpdateCurrentUser).Methods(http.MethodPut)

			group, done := errgroup.WithContext(ctx.Context)
			group.Go(func() error {
				listener, err := net.Listen("tcp", runConfig.BindAddress)
				if err != nil {
					return err
				}

				if tlsConfig != nil {
					log.Info("setting up tls")
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
