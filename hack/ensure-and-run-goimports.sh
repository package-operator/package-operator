#!/bin/bash
set -euo pipefail

export GOFLAGS=""
exec .deps/bin/goimports -local github.com/package-operator/package-operator -w -l "$@"
