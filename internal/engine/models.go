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
	"github.com/mjpitz/myago/pass"
)

// Permission defines a base permission applied to the system.
type Permission string

func (p Permission) String() string {
	for _, perm := range PermissionValues {
		if p == perm {
			return string(p)
		}
	}

	return ""
}

const (
	// ReadPermission grants a user read access to the system. For example, this allows SELECT statements to be issued
	// against SQL systems.
	ReadPermission Permission = "read"
	// WritePermission grants the user write access to a system. For example this allows INSERT statements to be issued
	// against SQL systems.
	WritePermission Permission = "write"
	// UpdatePermission grants the user permission to update the system. For example, this allows UPDATE statements to
	// be issued against the database.
	UpdatePermission Permission = "update"
	// DeletePermission grants the user delete access to the system. For example, this allows DELETE statements to be
	// issued against SQL systems.
	DeletePermission Permission = "delete"
	// AdminPermission grants the user admin access to the system. For example, this allows CREATE TABLE, ALTER TABLE,
	// and DROP TABLE statements to be issued against SQL systems.
	AdminPermission Permission = "admin"
	// SystemPermission is used to grant the user access to the GET /api/v1/credentials/{kind}/{name} endpoints, thus
	// allowing them to administer user accounts within the system. Granting this permission should only be used to
	// provide the connector with access to all the credentials that need to be added to the system.
	SystemPermission Permission = "system"
)

// PermissionValues defines an array of permissions within the system.
var PermissionValues = []Permission{
	ReadPermission, WritePermission, UpdatePermission, DeletePermission,
	AdminPermission, SystemPermission,
}

// Credentials defines derived credentials.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Templates define a set of templates used for generating usernames and passwords.
type Templates struct {
	UserTemplate     string `json:"user_template" usage:"configure which template class should be used for username generation [options: max,long,medium,short,basic,pin]"`
	PasswordTemplate string `json:"password_template" usage:"configure which template class should be used for password generation [options: max,long,medium,short,basic,pin]"`
}

type ServiceTemplates struct {
	UserTemplate     pass.TemplateClass `json:"user_template"`
	PasswordTemplate pass.TemplateClass `json:"password_template"`
}

// Service defines the various metadata of a service that is managed within varys.
type Service struct {
	Kind      string           `json:"kind"`
	Name      string           `json:"name"`
	Address   string           `json:"address"`
	Key       []byte           `json:"-"`
	Templates ServiceTemplates `json:"templates"`
}

// K returns a unique key for the service. Useful for caching in maps.
func (s Service) K() string {
	return "/_service/" + s.Kind + "/" + s.Name
}

// User represents a user within varys.
type User struct {
	Kind         string            `json:"kind"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	SiteCounters map[string]uint32 `json:"-"`
}

// K returns a unique key for the user. Useful for caching or referencing in maps.
func (u User) K() string {
	return "/_user/" + u.Kind + "/" + u.ID
}
