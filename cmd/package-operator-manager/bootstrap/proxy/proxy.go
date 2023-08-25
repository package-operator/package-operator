package proxy

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

const (
	httpProxyVar  = "HTTP_PROXY"
	httpsProxyVar = "HTTPS_PROXY"
	noProxyVar    = "NO_PROXY"
)

// typically `unix.Exec` but configurable to make code testable
// https://pkg.go.dev/golang.org/x/sys/unix#Exec
type ExecveFunc func(argv0 string, argv []string, envv []string) error

func RestartPKOWithEnvvarsIfNeeded(log logr.Logger, execve ExecveFunc, env *manifestsv1alpha1.PackageEnvironment) error {
	if env.Proxy == nil {
		log.Info("no proxy configured via PackageEnvironment")
		// no restart needed
		return nil
	}

	vars := proxyVars{
		httpProxy:  os.Getenv(httpProxyVar),
		httpsProxy: os.Getenv(httpsProxyVar),
		noProxy:    os.Getenv(noProxyVar),
	}

	if !vars.differentFrom(*env.Proxy) {
		log.Info("proxy settings in environment variables match those in PackageEnvironment config")
		// no restart needed
		return nil
	}

	vars.httpProxy = env.Proxy.HTTPProxy
	vars.httpsProxy = env.Proxy.HTTPSProxy
	vars.noProxy = env.Proxy.NoProxy

	log.Info("proxy settings in environment variables do not match those in PackageEnvironment config")
	log.Info("restarting with updated proxy envvars")
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	return execve(executable, os.Args, vars.mergeTo(os.Environ()))
}

// small helper struct to make code readable/testable.
type proxyVars struct {
	httpProxy, httpsProxy, noProxy string
}

// helper to compare against proxy object from PackageEnvironment.
func (pv proxyVars) differentFrom(proxy manifestsv1alpha1.PackageEnvironmentProxy) bool {
	return pv.httpProxy != proxy.HTTPProxy ||
		pv.httpsProxy != proxy.HTTPSProxy ||
		pv.noProxy != proxy.NoProxy
}

// merges proxy envvars with environ kv-string slice (overriding pre-existing variables) and returns the result.
func (pv proxyVars) mergeTo(environ []string) []string {
	merged := []string{}

	// copy all variables we don't override into result
	for _, kvstr := range environ {
		// don't copy proxy envvars
		if strings.HasPrefix(kvstr, fmt.Sprintf("%s=", httpProxyVar)) ||
			strings.HasPrefix(kvstr, fmt.Sprintf("%s=", httpsProxyVar)) ||
			strings.HasPrefix(kvstr, fmt.Sprintf("%s=", noProxyVar)) {
			continue
		}

		merged = append(merged, kvstr)
	}

	// add proxy envvars and return result
	return append(merged,
		fmt.Sprintf("%s=%s", httpProxyVar, pv.httpProxy),
		fmt.Sprintf("%s=%s", httpsProxyVar, pv.httpsProxy),
		fmt.Sprintf("%s=%s", noProxyVar, pv.noProxy),
	)
}
