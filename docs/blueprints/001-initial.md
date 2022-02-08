# Initial System Design

In this project, I'm going to try a new approach to the systems that I design. Generally speaking, I like to plan a
decent amount out before diving in. But in this case, I'm going to stop and intentionally build something simpler.

## Goals

- Provide a reasonably secure, easy to manage alternative to HashiCorp's Vault project.
- Leverage modern cryptographic practices ensuring secure key generation and derivation.
- Easily rotate keys per user, per service, or for the entire system.

## Implementation

This will start as an HTTP service. Later discussions can be had about adding other interfaces.

Since the default configuration is insecure by default, the process binds to localhost to avoid any unknowing security
leaks. When deploying on anything other than localhost, a more secure configuration should be used.

### Authentication

All requests to the system should be authenticated. Authentication will be handled by basic auth for the time being as
we expect all communication with the system to be encrypted. Basic auth can be performed using a username and password
combination or using a bearer token file.

If no file is specified (not recommended) then default credentials will be used (username: `badadmin`, password: 
`badadmin`).

**Username / Password File:**

```csv
"password","username","userID","group1,group2"
```

**Bearer Token File:**

```csv
"username","token","userID","group1"
```

**Environment Variables:**

- `VARYS_AUTH_TYPE` - specifies which auth type should be used (options: `basic`)
- `VARYS_BASIC_PASSWORD_FILE` - path to the csv file containing usernames and passwords
- `VARYS_BASIC_TOKEN_FILE` - path to the csv file containing tokens

### Authorization

[Casbin](https://casbin.org/) is an easy-to-use framework for handling access control within a system. This will allow
us to set up basic role based authorization. The permission structure is as follows:

- `read:varys:credentials:{service}:{name}` - allows the user to read all credentials for a service.
- `read:varys:services` - Allows the user to read services within the system.
- `write:varys:services` - Allows the user to create services within the system.
- `read:varys:users` - Allows the user to read users within the system.
- `admin:varys` - Allows the user to administer the system.
- `read:{service}` - Allows the user to read from all services of the given type.
- `write:{service}` - Allows the user to write to all services of the given type.
- `admin:{service}` - Allows the user to administer all services of the given type.
- `read:{service}:{name}` - Allows the user to read from the specific service.
- `write:{service}:{name}` - Allows the user to write to the specified service.
- `admin:{service}:{name}` - Allows the user to administer the specified service.

From these, we can craft our roles:

| Role                     | Permissions                               |
|--------------------------|-------------------------------------------|
| `read:varys`             | `read:varys:user`, `read:varys:site`      |
| `admin:varys`            | `write:varys:site`, `admin:varys`         |
| `read:{service}`         | `read:{service}`                          |
| `admin:{service}`        | `write:{service}`, `admin:{service}`      |
| `admin:{service}:{name}` | `read:varys:credentials:{service}:{name}` |

### Encryption in Transit

Authentication is performed using access tokens that are transported in plaintext as part of the request header. As a
result, all communication with `varys` should be encrypted. This allows credentials to be handed over securely.

**Environment Variables:**

- `VARYS_TLS_ENABLED`
- `VARYS_TLS_CERT_PATH`
- `VARYS_TLS_CA_FILE`
- `VARYS_TLS_CERT_FILE`
- `VARYS_TLS_KEY_FILE`

### Encryption at Rest

Even though we don't store any of the derived keys to disk, we still encrypt the data to protect it in the event the
disk is compromised. Handling rotating the encryption key is currently out of scope for this project.

Protect it wisely.

**Environment Variables:**

- `VARYS_DATABASE_ENCRYPTION_KEY`

### Credential Derivation

A key design to this system is that the resulting credentials are never stored within the system. They are derived on
the fly using well-known cryptographic functions. `varys` will build upon the work of the Spectre app algorithm
detailed [here](https://spectre.app/spectre-algorithm.pdf).

When deriving credentials for a service in `varys` we use a key (in the Spectre paper, this is called the master 
password) that's derived by mixing a root key and a service key. The service key is a cryptographically secure, 256bit 
random passphrase that is generated at time of creation in `varys`. By mixing the two keys, we now have two options for 
rotating credentials in `varys`. A new root key can be provided, causing every service to generate new credentials. Or a 
new service key can be generated, causing only the service to be regenerated.

In order to support per-user credential regeneration, we separate the counter storage from the service and persist it
alongside our user definition. To obtain a new credential for a service, we simply increment the counter stored on that
user record.

**Models:**

```go
package blueprint

import "github.com/mjpitz/myago/pass"

type User struct {
	ID           string
	Name         string
	SiteCounters map[string]uint32
}

type Credential struct {
	Username string
	Password string
}

type Templates struct {
	UserTemplate     pass.TemplateClass
	PasswordTemplate pass.TemplateClass
}

type Service struct {
	Name      string
	Address   string
	Key       []byte
	Templates Templates
}
```

**Environment Variables:**

- `VARYS_CREDENTIAL_ROOT_KEY`

### Endpoints

- `GET    /api/v1/credentials/{service}/{name}` returns a list of derived credentials for the service.
- `GET    /api/v1/credentials/{service}/{name}/self` returns derived credentials for the service for the current user.
- `GET    /api/v1/services` returns a list of services that `varys` is managing.
- `GET    /api/v1/services/{service}/{name}` returns basic information about the specified service.
- `POST   /api/v1/services/{service}/{name}` create or update the service with new information.
- `PUT    /api/v1/services/{service}/{name}` create or update the service with new information.
- `DELETE /api/v1/services/{service}/{name}` deletes the specified service.
- `GET    /api/v1/users` returns a list of known users in the system.
- `GET    /api/v1/users/self` returns information about the current user.
