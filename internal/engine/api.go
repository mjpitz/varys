package engine

import (
	"github.com/dgraph-io/badger/v3"
)

func NewAPI(db *badger.DB, root string) *API {
	return &API{
		db:   db,
		root: root,
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
	db   *badger.DB
	root string

	users    *Store
	services *Store
}
