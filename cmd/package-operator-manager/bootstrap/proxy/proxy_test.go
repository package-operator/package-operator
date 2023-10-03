package proxy

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestRestartPKOWithEnvvarsIfNeeded(t *testing.T) {
	t.Parallel()

	log := testr.New(t)

	// nothing should happen and nil functions should not be called when env.Proxy is empty (which means that no proxy is configured in the cluster)
	// TODO(jgwosdz): there is an edge case where PKO was started with envvars that configure a proxy but no proxy is configured in the cluster. Should pko then strip this proxy from the env and restart itself or should it just keep running with proxy settings?
	t.Run("env.Proxy_IsNil", func(t *testing.T) {
		t.Parallel()

		var err error
		require.NotPanics(t, func() {
			err = restartPKOWithEnvvarsIfNeeded(log, nil, nil, nil, &manifestsv1alpha1.PackageEnvironment{
				Proxy: nil,
			})
		})
		require.NoError(t, err)
	})

	t.Run("env.ProxyEqualsOsEnv", func(t *testing.T) {
		t.Parallel()

		httpProxy := "http://http-proxy"
		httpsProxy := "http://https-proxy"
		noProxy := "http://no-proxy"

		getenv := mockGetenv(map[string]string{
			httpProxyVar:  httpProxy,
			httpsProxyVar: httpsProxy,
			noProxyVar:    noProxy,
		})

		apiEnv := &manifestsv1alpha1.PackageEnvironment{
			Proxy: &manifestsv1alpha1.PackageEnvironmentProxy{
				HTTPProxy:  httpProxy,
				HTTPSProxy: httpsProxy,
				NoProxy:    noProxy,
			},
		}

		var err error
		require.NotPanics(t, func() {
			err = restartPKOWithEnvvarsIfNeeded(log, nil, getenv, nil, apiEnv)
		})
		require.NoError(t, err)
	})

	{
		mockedGetenv := mockGetenv(map[string]string{
			httpProxyVar:  "",
			httpsProxyVar: "",
			noProxyVar:    "",
		})

		expectedEnv := map[string]string{
			httpProxyVar:  "http://http-proxy",
			httpsProxyVar: "http://https-proxy",
			noProxyVar:    "http://no-proxy",
		}

		apiEnv := &manifestsv1alpha1.PackageEnvironment{
			Proxy: &manifestsv1alpha1.PackageEnvironmentProxy{
				HTTPProxy:  expectedEnv[httpProxyVar],
				HTTPSProxy: expectedEnv[httpsProxyVar],
				NoProxy:    expectedEnv[noProxyVar],
			},
		}

		t.Run("env.ProxyDiffersOsEnvButResolvingExecutableErrs", func(t *testing.T) {
			t.Parallel()

			errTest := errors.New("resolving executable") //nolint:goerr113
			mockedExecutable := mockExecutable("/irrelevant", errTest)

			var err error
			require.NotPanics(t, func() {
				err = restartPKOWithEnvvarsIfNeeded(log, nil, mockedGetenv, mockedExecutable, apiEnv)
			})
			require.ErrorIs(t, err, errTest)
		})

		t.Run("env.ProxyDiffersOsEnv", func(t *testing.T) {
			t.Parallel()

			expectedExecutablePath := "/random/path/for/testing"
			mockedExecutable := mockExecutable(expectedExecutablePath, nil)

			execveCalled := false
			execve := func(argv0 string, args []string, env []string) error {
				execveCalled = true

				assert.Equal(t, argv0, expectedExecutablePath)
				assert.Equal(t, reflect.DeepEqual(args, os.Args), true)

				envMap, err := parseEnvSlice(env)
				require.NoError(t, err)

				// assert that proxy envvars would be passed to execve
				for expectedKey, expectedValue := range expectedEnv {
					assert.Equal(t, envMap[expectedKey], expectedValue,
						"proxy envvars should be equal",
						"actual", envMap[expectedKey],
						"expected", expectedValue)
				}

				return nil
			}

			var err error
			require.NotPanics(t, func() {
				err = restartPKOWithEnvvarsIfNeeded(log, execve, mockedGetenv, mockedExecutable, apiEnv)
			})
			require.NoError(t, err)
			assert.Equal(t, execveCalled, true)
		})
	}
}

func TestProxyVarsDifferentFrom(t *testing.T) {
	t.Parallel()

	pv := proxyVars{
		httpProxy:  "foo",
		httpsProxy: "bar",
		noProxy:    "baz",
	}

	tcases := []struct {
		proxy    manifestsv1alpha1.PackageEnvironmentProxy
		expected bool
	}{
		{
			proxy: manifestsv1alpha1.PackageEnvironmentProxy{
				HTTPProxy:  "foo",
				HTTPSProxy: "bar",
				NoProxy:    "baz",
			},
			expected: false,
		},
		{
			proxy: manifestsv1alpha1.PackageEnvironmentProxy{
				HTTPProxy:  "http",
				HTTPSProxy: "https",
				NoProxy:    "no",
			},
			expected: true,
		},
	}

	for i := range tcases {
		tc := tcases[i]
		assert.Equal(t, pv.differentFrom(tc.proxy), tc.expected)
	}
}

func TestProxyVarsMergeTo(t *testing.T) {
	t.Parallel()

	pv := proxyVars{
		httpProxy:  "foo",
		httpsProxy: "bar",
		noProxy:    "baz",
	}

	tcases := []struct {
		env      []string
		expected []string
	}{
		{
			env: []string{"old_var=val"},
			expected: []string{
				"old_var=val",
				fmt.Sprintf("%s=%s", httpProxyVar, pv.httpProxy),
				fmt.Sprintf("%s=%s", httpsProxyVar, pv.httpsProxy),
				fmt.Sprintf("%s=%s", noProxyVar, pv.noProxy),
			},
		},
		{
			env: []string{fmt.Sprintf("%s=old-proxy", httpProxyVar), "old_var=val"},
			expected: []string{
				"old_var=val",
				fmt.Sprintf("%s=%s", httpProxyVar, pv.httpProxy),
				fmt.Sprintf("%s=%s", httpsProxyVar, pv.httpsProxy),
				fmt.Sprintf("%s=%s", noProxyVar, pv.noProxy),
			},
		},
	}

	for i := range tcases {
		tc := tcases[i]

		assert.Equal(t, reflect.DeepEqual(pv.mergeTo(tc.env), tc.expected), true)
	}
}

// TEST HELPERS BELOW

var errSplitEnv = errors.New("splitting entry into key and value")

// Does not check for duplicate entries. Last entry wins!
func parseEnvSlice(slice []string) (map[string]string, error) {
	env := map[string]string{}

	for _, entry := range slice {
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("%w: %s, %+v", errSplitEnv, entry, kv)
		}
		env[kv[0]] = kv[1]
	}
	return env, nil
}

func mockExecutable(executable string, err error) func() (string, error) {
	return func() (string, error) {
		return executable, err
	}
}

func mockGetenv(env map[string]string) getenvFn {
	return func(key string) string {
		return env[key]
	}
}

func TestHelperParseEnvSlice(t *testing.T) {
	t.Parallel()

	tcases := []struct {
		slice    []string
		expected map[string]string
		errIs    error
	}{
		{
			slice: []string{"A=1", "B=2", "C=three"},
			expected: map[string]string{
				"A": "1",
				"B": "2",
				"C": "three",
			},
		},
		{
			// not correctly formatted k=v pair
			slice: []string{"A:1"},
			errIs: errSplitEnv,
		},
	}

	for i := range tcases {
		tc := tcases[i]
		actual, err := parseEnvSlice(tc.slice)
		if tc.errIs != nil {
			require.ErrorIs(t, err, tc.errIs)
		} else {
			require.NoError(t, err)
		}
		for expectedKey, expectedValue := range tc.expected {
			assert.Equal(t, actual[expectedKey], expectedValue,
				"actual[expectedKey] and expectedValue should be equal",
				"actual", actual[expectedKey],
				"expected", expectedValue)
		}
	}
}
