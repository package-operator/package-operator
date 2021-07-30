package integration_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"

	"github.com/openshift/addon-operator/integration"
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	defer func() {
		if err := integration.PrintPodStatusAndLogs("addon-operator"); err != nil {
			log.Fatal(err)
		}
	}()

	// Setup
	setupExitCode := testing.MainStart(&deps{}, []testing.InternalTest{
		{
			Name: "Setup",
			F:    Setup,
		},
	}, nil, nil).Run()
	if setupExitCode != 0 {
		return setupExitCode
	}
	fmt.Println()

	// Main tests
	exitCode := m.Run()
	if exitCode != 0 {
		return exitCode
	}
	fmt.Println()

	// Teardown
	teardownExitCode := testing.MainStart(&deps{}, []testing.InternalTest{
		{
			Name: "Teardown",
			F:    Teardown,
		},
	}, nil, nil).Run()
	return teardownExitCode
}

type deps struct{}

func (*deps) ImportPath() string { return "" }

func (*deps) MatchString(pat, str string) (bool, error) {
	return true, nil
}

func (*deps) SetPanicOnExit0(bool) {}

func (*deps) StartCPUProfile(io.Writer) error { return nil }

func (*deps) StopCPUProfile() {}

func (*deps) StartTestLog(wr io.Writer) {}

func (*deps) StopTestLog() error { return nil }

func (*deps) WriteProfileTo(string, io.Writer, int) error { return nil }
