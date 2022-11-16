package version

import (
	"runtime/debug"
)

// Info contains build information supplied during compile time.
type Info struct {
	*debug.BuildInfo
	Version string `json:"version"`
}

// version gets filled by a linker argument and should contain the app version.
var version string

// Get version related embedded information.
func Get() Info {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		panic("no build info available: app was build without module support... how did you manage that? 🤨")
	}

	return Info{buildInfo, version}
}
