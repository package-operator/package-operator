#!/usr/bin/env bash

set -euo pipefail

# Core API Group
# --------------
./.deps/bin/k8s-docgen apis/core/v1alpha1 > ./docs/api-reference.md
