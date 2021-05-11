#!/bin/bash
set -euo pipefail

# this script ensures that the `goimports` dependency is present
# and then executes goimport passing all arguments forward

make -s goimports
.cache/dependencies/bin/goimports -local github.com/openshift/addon-operator -w -l "$@"
