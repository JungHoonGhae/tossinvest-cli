package version

import "strings"

// Repo is the canonical GitHub "owner/repo" slug — the single source of truth
// for every runtime reference to this project's repository (release links,
// update-check API, schema URL). Change it here once and they all follow.
//
// NOTE: Go *import paths* (github.com/JungHoonGhae/tossinvest-cli/...) cannot
// use this — import paths are compile-time string literals fixed by the
// language, so a repo rename still requires a module-path find-replace across
// all files. This constant only centralizes runtime URLs.
const Repo = "JungHoonGhae/tossinvest-cli"

// ReleasesLatestURL is the human-facing "latest release" page.
const ReleasesLatestURL = "https://github.com/" + Repo + "/releases/latest"

// RawMainURL builds a raw.githubusercontent.com URL for a file on the main
// branch (e.g. RawMainURL("schemas/config.schema.json")).
func RawMainURL(path string) string {
	return "https://raw.githubusercontent.com/" + Repo + "/main/" + path
}

// RawTagURL builds a raw.githubusercontent.com URL for a file at release tag
// v<ver> (e.g. RawTagURL("install.sh", "0.54.1")). A leading "v" in ver is
// tolerated. Prefer this over RawMainURL for anything executed: main moves,
// tags don't.
func RawTagURL(path, ver string) string {
	ver = strings.TrimPrefix(ver, "v")
	return "https://raw.githubusercontent.com/" + Repo + "/v" + ver + "/" + path
}

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date,omitempty"`
}

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = ""
)

func Current() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
}
