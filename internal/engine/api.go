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
	"github.com/casbin/casbin/v2"
	"github.com/dgraph-io/badger/v3"
)

// NewAPI constructs a new API definition used to mount the various endpoints for the engine.
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

// API encapsulates the requirements of operating the API.
type API struct {
	db       *badger.DB
	enforcer *casbin.Enforcer
	root     string

	users    *Store
	services *Store
}
