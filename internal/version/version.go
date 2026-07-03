// Package version holds build metadata for the VexPay binary.
package version

// These are overridden at build time via -ldflags.
var (
	// Version is the semantic version of the build.
	Version = "0.0.0-dev"
	// Commit is the git commit the binary was built from.
	Commit = "unknown"
	// BuildDate is the RFC3339 build timestamp.
	BuildDate = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return Version + " (" + Commit + ", built " + BuildDate + ")"
}
