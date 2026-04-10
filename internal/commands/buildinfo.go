package commands

// build metadata set from main via SetBuildInfo before Execute (linker -X or defaults).
var buildVersion, buildCommit, buildDate string

// SetBuildInfo records release metadata from the main package. Call from main() before Execute.
func SetBuildInfo(version, commit, date string) {
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = "none"
	}
	if date == "" {
		date = "unknown"
	}
	buildVersion = version
	buildCommit = commit
	buildDate = date
	rootCmd.Version = buildVersion
}

// BuildVersion returns the embedded version (e.g. v0.2.0 or dev).
func BuildVersion() string {
	if buildVersion == "" {
		return "dev"
	}
	return buildVersion
}

// BuildCommit returns the embedded git commit short hash.
func BuildCommit() string {
	if buildCommit == "" {
		return "none"
	}
	return buildCommit
}

// BuildDate returns the embedded build timestamp.
func BuildDate() string {
	if buildDate == "" {
		return "unknown"
	}
	return buildDate
}
