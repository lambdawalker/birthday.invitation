#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"
if ! command -v go >/dev/null 2>&1; then
  printf '%s\n' 'Go is required. Install it from https://go.dev/dl/ and run this script again.' >&2
  exit 1
fi
exec go run ./cmd/deploy "$@"
