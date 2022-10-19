package main

import "fmt"

var (
	ErrPackageNameNotFound      = fmt.Errorf("packageName found to be empty")
	ErrPackageNamespaceNotFound = fmt.Errorf("packageNamespace found to be empty")
	ErrPackageDirNotFound       = fmt.Errorf("packageDir found to be empty")
	ErrScopeNotFound            = fmt.Errorf("scope found to be empty")
)
