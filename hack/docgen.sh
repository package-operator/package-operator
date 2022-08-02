#!/usr/bin/env bash

set -euo pipefail

cat << 'EOF' > ./docs/api-reference.md
# Package Operator API Reference

The Package Operator APIs are an extension of the [Kubernetes API](https://kubernetes.io/docs/reference/using-api/api-overview/) using `CustomResourceDefinitions`.

EOF

# Core API Group
# --------------
./.deps/bin/k8s-docgen apis/core/v1alpha1 >> ./docs/api-reference.md
