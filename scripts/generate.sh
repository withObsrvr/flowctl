#!/usr/bin/env bash
#
# Regenerate Go code from the protobuf definitions.
#
# buf is the single generator for this repo, with plugin versions pinned in
# buf.gen.yaml so that regenerating produces identical bytes everywhere. This
# used to call a locally installed protoc, which made the output depend on
# whichever plugin versions a developer happened to have.
#
# Equivalent to `make generate`.

set -euo pipefail

cd "$(dirname "$0")/.."
exec go run github.com/bufbuild/buf/cmd/buf@v1.47.2 generate
