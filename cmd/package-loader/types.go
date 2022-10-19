package main

import (
	"fmt"
	"strings"
)

type mapFlags map[string]string

func (m mapFlags) String() string {
	output := ""
	for k, v := range m {
		output += fmt.Sprintf("%s=%s,", k, v)
	}
	return strings.TrimSuffix(output, ",")
}

func (m *mapFlags) Set(input string) error {
	*m = map[string]string{}
	labels := strings.Split(input, ",")
	for _, label := range labels {
		kvPair := strings.Split(label, "=")
		if len(kvPair) != 2 {
			return fmt.Errorf("improper label '%s' found in the labels input '%s'", label, input) //nolint:goerr113
		}
		key, value := kvPair[0], kvPair[1]
		(*m)[key] = value
	}
	return nil
}

type scopeFlags string

func (s scopeFlags) String() string {
	return string(s)
}

func (s *scopeFlags) Set(input string) error {
	*s = scopeFlags(input)
	return nil
}

type loaderOpts struct {
	packageName      string
	packageNamespace string
	packageDir       string
	ensureNamespace  bool
	labels           mapFlags
	scope            scopeFlags
	debugMode        bool
}

func (l loaderOpts) isValid() error {
	if l.packageName == "" {
		return ErrPackageNameNotFound
	}
	if l.scope == "" {
		return ErrScopeNotFound
	}
	if l.scope == namespaceScope && l.packageNamespace == "" {
		return ErrPackageNamespaceNotFound
	}
	if l.packageDir == "" {
		return ErrPackageDirNotFound
	}

	return nil
}

const (
	packageManifestFileName = "manifest.yaml"

	packageOperatorPhaseAnnotation = "package-operator.run/phase"

	clusterScope   = "cluster"
	namespaceScope = "namespace"
)
