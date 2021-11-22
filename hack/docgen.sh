#!/usr/bin/env bash

set -euo pipefail

cat << 'EOF' > ./docs/api-reference/_index.md
---
title: API Reference
weight: 50
---

# Addon Operator API Reference

The Addon Operator APIs are an extension of the [Kubernetes API](https://kubernetes.io/docs/reference/using-api/api-overview/) using `CustomResourceDefinitions`.

EOF

# Addons API Group
# --------------
cat << 'EOF' >> ./docs/api-reference/_index.md
## `addons.managed.openshift.io`

The `addons.managed.openshift.io` API group in managed OpenShift contains all Addon related API objects.

EOF
find ./apis/addons/v1alpha1 -name '*types.go'  | xargs ./bin/docgen >> ./docs/api-reference/_index.md
