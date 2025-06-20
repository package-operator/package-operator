package version

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"
)

// Info contains compile time info about build binaries.
//
// This currently takes the BuildInfo from the [debug.BuildInfo] and the application version
// added by mage.
type Info struct {
	*debug.BuildInfo
	ApplicationVersion string `json:"version"`
}

// version gets filled by a linker argument and should contain the app version.
var version string

// Get [Info] for the current runtime.
func Get() Info {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		panic("no build info available: app was build without module support... how did you manage that? 🤨")
	}

	return Info{buildInfo, version}
}

// Write the given version Info to parameter out. Returns any errors the writer returns.
func (i Info) Write(out io.Writer) error {
	buf := &bytes.Buffer{}

	if i.ApplicationVersion != "" {
		_, _ = fmt.Fprintf(buf, "pko\t%s\n", i.ApplicationVersion)
	}

	_, _ = fmt.Fprint(buf, i.String())

	if _, err := buf.WriteTo(out); err != nil {
		return fmt.Errorf("write version info to output writer: %w", err)
	}

	return nil
}
