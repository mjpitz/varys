# varys

There have been several times in the last year when I wanted a somewhat simpler solution to HashiCorp's Vault project.
Don't get me wrong, I absolutely love the product and have used it on several occasions at work. But for smaller
projects, administering it can be somewhat of a hassle.

And so I decided to build `varys`, a tool for deriving secrets and managing privileged access to services. Unlike
Vault, `varys` doesn't store any secrets on disk. Instead, credentials are derived on the fly and require authorization
to the service in order to obtain them.

## Status

[![Status: MVP][status-img]][status-link]
[![License: AGPL-3.0][license-img]][license-link]

[status-img]: https://img.shields.io/badge/Status-MVP-lightgrey?style=flat-square
[status-link]: docs/blueprints

[license-img]: https://img.shields.io/github/license/mjpitz/varys?label=License&style=flat-square
[license-link]: LICENSE

### Features

* All requests require authentication and authorization.
* Data is encrypted in transit and at rest.
* Easily rotate keys per user, per service, or for all services within `varys`.
* Derived secrets are never persisted within the system, only some metadata used to derive them.

## Getting Started

For now, you'll need to install `varys` the old-fashion way.

```
$ go install github.com/mjpitz/varys/cmd/varys@latest
```

## Resources

- [Documentation](https://github.com/mjpitz/varys/wiki)
  - [Configuration](https://github.com/mjpitz/varys/wiki/Configuration)
  - [Running a server](https://github.com/mjpitz/varys/wiki/Running-a-server)
  - [Using the CLI](https://github.com/mjpitz/varys/wiki/Using-the-CLI)
- [License](LICENSE)
