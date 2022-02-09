package engine

import (
	"github.com/casbin/casbin/v2"
	"github.com/dgraph-io/badger/v3"
)

func NewAPI(db *badger.DB, enforcer *casbin.Enforcer, root string) *API {
	return &API{
		db:       db,
		enforcer: enforcer,
		root:     root,
		users: &Store{
			db:     db,
			prefix: "varys/users",
		},
		services: &Store{
			db:     db,
			prefix: "varys/services",
		},
	}
}

type API struct {
	db       *badger.DB
	enforcer *casbin.Enforcer
	root     string

	users    *Store
	services *Store
}
