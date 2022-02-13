CWD = $(shell pwd)
SKAFFOLD_DEFAULT_REPO ?= ghcr.io/mjpitz
VERSION ?= latest

define HELP_TEXT
Welcome to varys!

Targets:
  help      provides help text
  test      run tests
  legal     prepends legal header to source code
  dist      distributes the binaries

endef
export HELP_TEXT

help:
	@echo "$$HELP_TEXT"

docker: .docker
.docker:
	docker build . \
		--tag $(SKAFFOLD_DEFAULT_REPO)/varys:latest \
		--tag $(SKAFFOLD_DEFAULT_REPO)/varys:$(VERSION) \
		--file ./cmd/varys/Dockerfile

docker/release:
	docker buildx build . \
		--platform linux/amd64,linux/arm64 \
		--label "org.opencontainers.image.source=https://github.com/mjpitz/varys" \
		--label "org.opencontainers.image.version=$(VERSION)" \
		--label "org.opencontainers.image.licenses=AGPL-3.0" \
		--label "org.opencontainers.image.title=varys" \
		--label "org.opencontainers.image.description=A derivation-based secret engine and privileged access management system" \
		--tag $(SKAFFOLD_DEFAULT_REPO)/varys:latest \
		--tag $(SKAFFOLD_DEFAULT_REPO)/varys:$(VERSION) \
		--file ./cmd/varys/Dockerfile \
		--push

# actual targets

test:
	go test -v -race -coverprofile=.coverprofile -covermode=atomic ./...
legal: .legal
.legal:
	addlicense -f ./legal/header.txt -skip yaml -skip yml .

dist: .dist
.dist:
	sh ./scripts/dist-go.sh

# useful shortcuts for release

tag/release:
	npm version "$(shell date +%y.%m.0)"
	git push --follow-tags

tag/patch:
	npm version patch
	git push --follow-tags
