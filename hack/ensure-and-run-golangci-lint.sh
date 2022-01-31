#!/bin/bash
set -euo pipefail

# this script ensures that the `golangci-lint` dependency is present
# and then executes goimport passing all arguments forward to the `run` command

./mage dependency:golangcilint
export GOFLAGS=""
exec .deps/bin/golangci-lint run "$@"
