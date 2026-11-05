package version

import (
	"strings"
	"testing"
)

func TestRawTagURLPinsReleaseTag(t *testing.T) {
	for _, ver := range []string{"0.54.1", "v0.54.1"} {
		got := RawTagURL("install.sh", ver)
		want := "https://raw.githubusercontent.com/" + Repo + "/v0.54.1/install.sh"
		if got != want {
			t.Fatalf("RawTagURL(install.sh, %q) = %q, want %q", ver, got, want)
		}
		if strings.Contains(got, "/main/") {
			t.Fatalf("RawTagURL(%q) points at mutable main: %q", ver, got)
		}
	}
}
