#!/usr/bin/env bash

set -e -o pipefail

go mod download
go mod verify

if [[ -z "${VERSION}" ]]; then
	goreleaser --snapshot --skip-publish --rm-dist
else
	goreleaser
fi

rm -rf "$(pwd)/varys"

os=$(uname | tr '[:upper:]' '[:lower:]')
arch="$(uname -m)"
if [[ "$arch" == "x86_64" ]]; then
	ln -s "$(pwd)/dist/varys_${os}_amd64/varys" "$(pwd)/varys"
elif [[ "$arch" == "aarch64" ]]; then
	ln -s "$(pwd)/dist/varys_${os}_arm64/varys" "$(pwd)/varys"
fi
