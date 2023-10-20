package proxy

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/sys/unix"

	"package-operator.run/internal/apis/manifests"
)

const (
	httpProxyVar  = "HTTP_PROXY"
	httpsProxyVar = "HTTPS_PROXY"
	noProxyVar    = "NO_PROXY"
)

// Compare proxy settings in environment variables with settings in `apiEnv` and replace current pko process with a new instance with updated environment variables by using unix.Exec (execve).
// This is needed as a workaround because Go caches proxy settings that are read from the envvars `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` on the first global http(s) client call.
func RestartPKOWithEnvvarsIfNeeded(log logr.Logger, apiEnv *manifests.PackageEnvironment) error {
	return restartPKOWithEnvvarsIfNeeded(
		log,
		unix.Exec,
		os.Getenv,
		os.Executable,
		apiEnv,
	)
}

type (
	// Typically `unix.Exec` but configurable to make code testable.
	// https://pkg.go.dev/golang.org/x/sys/unix#Exec
	execveFn func(argv0 string, argv []string, envv []string) error

	// Typically `os.Getenv` but configurable to make code testable.
	getenvFn func(key string) string

	// Typically `os.Executable` but configurable to make code testable.
	executableFn func() (string, error)
)

func restartPKOWithEnvvarsIfNeeded(log logr.Logger, execve execveFn, getenv getenvFn, getAndResolveArgv0 executableFn, apiEnv *manifests.PackageEnvironment) error {
	if apiEnv.Proxy == nil {
		log.Info("no proxy configured via PackageEnvironment")
		// no restart needed
		return nil
	}

	vars := proxyVars{
		httpProxy:  getenv(httpProxyVar),
		httpsProxy: getenv(httpsProxyVar),
		noProxy:    getenv(noProxyVar),
	}

	if !vars.differentFrom(*apiEnv.Proxy) {
		log.Info("proxy settings in environment variables match those in PackageEnvironment config")
		// no restart needed
		return nil
	}

	vars.httpProxy = apiEnv.Proxy.HTTPProxy
	vars.httpsProxy = apiEnv.Proxy.HTTPSProxy
	vars.noProxy = apiEnv.Proxy.NoProxy

	log.Info("proxy settings in environment variables do not match those in PackageEnvironment config")
	log.Info("restarting with updated proxy envvars")
	executable, err := getAndResolveArgv0()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	return execve(executable, os.Args, vars.mergeTo(os.Environ()))
}

// small helper struct to make code readable/testable.
type proxyVars struct {
	httpProxy, httpsProxy, noProxy string
}

// Compare against proxy object from PackageEnvironment.
func (pv proxyVars) differentFrom(proxy manifests.PackageEnvironmentProxy) bool {
	return pv.httpProxy != proxy.HTTPProxy ||
		pv.httpsProxy != proxy.HTTPSProxy ||
		pv.noProxy != proxy.NoProxy
}

// Merge proxy envvars with environ kv-string slice (overriding pre-existing variables) and return the result.
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
