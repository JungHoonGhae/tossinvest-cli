// internal/selfupdate/method_test.go
package selfupdate

import "testing"

func TestDetectInstallMethod(t *testing.T) {
	cases := []struct {
		name     string
		execPath string
		version  string
		want     InstallMethod
	}{
		{"dev build", "/usr/local/bin/tossctl", "dev", MethodDev},
		{"dev build even under cellar path", "/opt/homebrew/Cellar/tossctl-cli/0.14.0/bin/tossctl", "dev", MethodDev},
		{"homebrew apple silicon", "/opt/homebrew/Cellar/tossctl-cli/0.14.0/bin/tossctl", "0.14.0", MethodHomebrew},
		{"homebrew intel", "/usr/local/Cellar/tossctl-cli/0.14.0/bin/tossctl", "0.14.0", MethodHomebrew},
		{"homebrew linuxbrew", "/home/linuxbrew/.linuxbrew/Cellar/tossctl-cli/0.14.0/bin/tossctl", "0.14.0", MethodHomebrew},
		{"install.sh binary in usr local bin", "/usr/local/bin/tossctl", "0.14.0", MethodBinary},
		{"windows install.ps1 binary", `C:\Users\me\AppData\Local\tossctl\tossctl.exe`, "0.14.0", MethodBinary},
		{"custom INSTALL_DIR binary", "/home/me/.local/bin/tossctl", "0.14.0", MethodBinary},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectInstallMethod(tc.execPath, tc.version)
			if got != tc.want {
				t.Errorf("DetectInstallMethod(%q, %q) = %v, want %v", tc.execPath, tc.version, got, tc.want)
			}
		})
	}
}

func TestInstallMethodString(t *testing.T) {
	cases := []struct {
		m    InstallMethod
		want string
	}{
		{MethodDev, "dev"},
		{MethodHomebrew, "homebrew"},
		{MethodBinary, "binary"},
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}
