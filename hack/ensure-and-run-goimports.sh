#!/bin/bash
set -euo pipefail

# this script ensures that the `goimports` dependency is present
# and then executes goimport passing all arguments forward

./mage dependency:goimports
export GOFLAGS=""
exec .deps/bin/goimports -local github.com/openshift/addon-operator -w -l "$@"
