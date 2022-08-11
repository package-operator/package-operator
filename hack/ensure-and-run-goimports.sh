#!/bin/bash
set -euo pipefail

export GOFLAGS=""
exec .deps/bin/goimports -local package-operator.run -w -l "$@"
