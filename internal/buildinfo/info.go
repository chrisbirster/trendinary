package buildinfo

import "strings"

const APIVersion = "v1"

// Release and Commit are populated by production builds through -ldflags.
// Local/test builds intentionally retain deterministic development values.
var (
	Release = "dev"
	Commit  = "unknown"
)

type Info struct {
	APIVersion string `json:"api_version"`
	Release    string `json:"release"`
	Commit     string `json:"commit"`
}

func Current() Info {
	return Normalize(Info{APIVersion: APIVersion, Release: Release, Commit: Commit})
}

func Normalize(info Info) Info {
	info.APIVersion = strings.TrimSpace(info.APIVersion)
	info.Release = strings.TrimSpace(info.Release)
	info.Commit = strings.TrimSpace(info.Commit)
	if info.APIVersion == "" {
		info.APIVersion = APIVersion
	}
	if info.Release == "" {
		info.Release = "dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	return info
}
