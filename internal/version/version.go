package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of the application
	Version = "1.0.0"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// BuildDate is the date when the binary was built
	BuildDate = "unknown"
	// GoVersion is the version of Go used to build the binary
	GoVersion = runtime.Version()
)

// Info contains version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetVersionInfo returns version information
func GetVersionInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	info := GetVersionInfo()
	return fmt.Sprintf("Redis to Valkey Migration Tool v%s\n"+
		"Git Commit: %s\n"+
		"Build Date: %s\n"+
		"Go Version: %s\n"+
		"Platform: %s",
		info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.Platform)
}
