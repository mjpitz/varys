package engine

import (
	"github.com/mjpitz/myago/pass"
)

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

// User represents a user within varys.
type User struct {
	Kind         string            `json:"kind"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	SiteCounters map[string]uint32 `json:"-"`
}
