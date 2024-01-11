package v1beta1

import (
	"fmt"
	"strings"
)

// From
// https://github.com/openshift/hypershift/blob/
// 9c3e998b0b37bedce07163a197e0bf30339e627e/hypershift-operator/
// controllers/manifests/manifests.go#L13.
func HostedClusterNamespace(cluster HostedCluster) string {
	return fmt.Sprintf("%s-%s", cluster.Namespace, strings.ReplaceAll(cluster.Name, ".", "-"))
}
