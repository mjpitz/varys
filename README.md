# varys

There have been several times in the last year when I wanted a somewhat simpler solution to HashiCorp's Vault project.
Don't get me wrong, I absolutely love the product and have used it on several occasions at work. But for smaller
projects, administering it can be somewhat of a hassle.

And so, I decided to build `varys`, a prototype secret engine that uses key derivation to synthesize credentials to a
system. `varys` acts as a system operator / administrator and ensures managed systems have the correct credentials.

## Status

- [LICENSE](LICENSE)
- [Blueprints](docs/blueprints)
