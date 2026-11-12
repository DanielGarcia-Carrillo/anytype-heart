#!/usr/bin/env bash
# Check a generated main of anytype-heart: everything builds, and the tests of
# every package the fork's patches touch pass. Run from the checkout of the result.
set -euo pipefail
base=${FORK_BASE:-refs/fork-main/upstream}

v=$(awk '$1 == "github.com/anyproto/tantivy-go" {print $2}' go.mod)
mkdir -p deps/libs/linux-amd64-musl
curl -fsSL "https://github.com/anyproto/tantivy-go/releases/download/$v/linux-amd64-musl.tar.gz" | tar -C deps/libs/linux-amd64-musl -xz

echo "::group::go build ./..."
go build ./...
echo "::endgroup::"

pkgs=$(git diff --name-only "$base" HEAD -- '*.go' | xargs -r -n1 dirname | sort -u | while read -r d; do [ -d "$d" ] && echo "./$d"; done)
if [ -z "$pkgs" ]; then
	echo "no Go packages changed by the fork's patches"
	exit 0
fi
echo "::group::go test (packages changed by the fork)"
echo "$pkgs"
go test $pkgs
echo "::endgroup::"
