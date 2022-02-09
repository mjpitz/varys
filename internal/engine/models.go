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
	// WritePermission grants the user write access to a system. For example this allows INSERT and UPDATE statements to
	// be issued against SQL systems.
	WritePermission Permission = "write"
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
var PermissionValues = []Permission{ReadPermission, WritePermission, DeletePermission, AdminPermission, SystemPermission}

// Credentials defines derived credentials.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Templates define a set of templates used for generating usernames and passwords.
type Templates struct {
	UserTemplate     pass.TemplateClass `json:"user_template"`
	PasswordTemplate pass.TemplateClass `json:"password_template"`
}

// Service defines the various metadata of a service that is managed within varys.
type Service struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Key       []byte    `json:"-"`
	Templates Templates `json:"templates"`
}

// K returns a unique key for the service. Useful for caching in maps.
func (s Service) K() string {
	return s.Kind + "/" + s.Name
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
	return u.Kind + "/" + u.ID
}
