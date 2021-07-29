#!/bin/bash
set -euo pipefail

# this script ensures that the `golangci-lint` dependency is present
# and then executes goimport passing all arguments forward to the `run` command

make -s golangci-lint

exec .cache/dependencies/bin/golangci-lint run "$@"
