# varys

There have been several times in the last year when I wanted a somewhat simpler solution to HashiCorp's Vault project.
Don't get me wrong, I absolutely love the product and have used it on several occasions at work. But for smaller
projects, administering it can be somewhat of a hassle.

And so I decided to build `varys`, a tool for deriving secrets and managing privileged access to services. Unlike
Vault, `varys` doesn't store any secrets on disk. Instead, credentials are derived on the fly and require authorization
to the service in order to obtain them.

## Status

[![Status: Initial Development][status-img]][status-link]
[![License: AGPL-3.0][license-img]][license-link]

[status-img]: https://img.shields.io/badge/Status-Initial%20Development-lightgrey?style=flat-square
[status-link]: docs/blueprints

[license-img]: https://img.shields.io/github/license/mjpitz/varys?label=License&style=flat-square
[license-link]: LICENSE

### Features

- [x] Authentication (basic)
- [ ] Authorization (casbin)
- [x] Encryption in transit
- [x] Encryption at rest
- [x] Endpoints (most)

## Getting Started

For now, you'll need to install `varys` the old-fashion way.

```
# go install github.com/mjpitz/varys/cmd/varys@latest

# varys run -h
NAME:
   main run - Runs the varys server process

USAGE:
   varys run [OPTIONS]

OPTIONS:
   --bind_address value                      specify the address to bind to (default: "localhost:3456") [$VARYS_BIND_ADDRESS]
   --tls_enable                              whether or not TLS should be enabled (default: false) [$VARYS_TLS_ENABLE]
   --tls_cert_path value                     where to locate certificates for communication [$VARYS_TLS_CERT_PATH]
   --tls_ca_file value                       override the ca file name (default: "ca.crt") [$VARYS_TLS_CA_FILE]
   --tls_cert_file value                     override the cert file name (default: "tls.crt") [$VARYS_TLS_CERT_FILE]
   --tls_key_file value                      override the key file name (default: "tls.key") [$VARYS_TLS_KEY_FILE]
   --tls_reload_interval value               how often to reload the config (default: 5m0s) [$VARYS_TLS_RELOAD_INTERVAL]
   --database_path value                     configure the path to the database (default: "db.badger") [$VARYS_DATABASE_PATH]
   --database_encryption_key value           specify the root encryption key used to encrypt the database [$VARYS_DATABASE_ENCRYPTION_KEY]
   --database_encryption_key_duration value  how long a derived encryption key is good for (default: 120h0m0s) [$VARYS_DATABASE_ENCRYPTION_KEY_DURATION]
   --credential_root_key value               specify the root key used to derive credentials from [$VARYS_CREDENTIAL_ROOT_KEY]
   --auth_type value                         configure the user authentication type to use [$VARYS_AUTH_TYPE]
   --basic_password_file value               path to the csv file containing usernames and passwords [$VARYS_BASIC_PASSWORD_FILE]
   --basic_token_file value                  path to the csv file containing tokens [$VARYS_BASIC_TOKEN_FILE]
   --help, -h                                show help (default: false)

```

### Default, insecure configuration

By default, `varys` runs in a semi-insecure mode.

- We `sha256` the provided encryption key to ensure a proper length. So even if an encryption key isn't provided, we 
  still end up using a non-empty encryption key.
- If no auth type is set, then a set of default credentials are used (username: `badadmin`, password: `badadmin`).
- Communication with the service is insecure by default. As a result, we bind the service to localhost to avoid any 
  unintended exposure. Its highly recommended that TLS is enabled before exposing the service beyond localhost.

### Secure configuration

It's highly recommended that `varys` is not run using the default configuration.

- Setting the `VARYS_DATABASE_ENCRYPTION_KEY` environment variable will configure a custom encryption key for the 
  database. This is provided directly to the underlying badger instance.
- Changing the `VARYS_CREDENTIAL_ROOT_KEY` allows keys to be quickly rotated for the entire application. This value is
  mixed with service keys, and user service counters, allowing credentials to be rotated at different levels of 
  granularity.
- `VARYS_AUTH_TYPE` should be set to `basic` (until `oidc` is supported). In addition to setting the auth type, you
  must specify either `VARYS_BASIC_PASSWORD_FILE` or `VARYS_BASIC_TOKEN_FILE`. This will configure what kind of basic
  authentication is used (either `Basic` or `Bearer` respectively).
- Since we require authentication and pass credentials as part of the header, `VARYS_TLS_ENABLE` and 
  `VARYS_TLS_CERT_PATH` should be set to configure secure communication with the service. In addition to configuring 
  TLS, you'll also want to set `VARYS_BIND_ADDRESS` to `0.0.0.0:3456` to bind the service outside of localhost.
