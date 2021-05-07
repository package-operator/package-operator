package e2e_test

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup
	setupExitCode := testing.MainStart(&deps{}, []testing.InternalTest{
		{
			Name: "Setup",
			F:    Setup,
		},
	}, nil, nil).Run()
	if setupExitCode != 0 {
		os.Exit(setupExitCode)
	}

	fmt.Println()

	// Main tests
	exitCode := m.Run()
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	fmt.Println()

	// Teardown
	teardownExitCode := testing.MainStart(&deps{}, []testing.InternalTest{
		{
			Name: "Teardown",
			F:    Teardown,
		},
	}, nil, nil).Run()
	os.Exit(teardownExitCode)
}

type deps struct{}

func (*deps) ImportPath() string {
	return ""
}

func (*deps) MatchString(pat, str string) (bool, error) {
	return true, nil
}

func (*deps) SetPanicOnExit0(bool) {}

func (*deps) StartCPUProfile(io.Writer) error {
	return nil
}

func (*deps) StopCPUProfile() {}

func (*deps) StartTestLog(wr io.Writer) {

}

func (*deps) StopTestLog() error {
	return nil
}

func (*deps) WriteProfileTo(string, io.Writer, int) error {
	return nil
}
