package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/doctor"
)

func WriteDoctorReport(w io.Writer, format Format, report doctor.Report) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case FormatCSV:
		return fmt.Errorf("csv output is not supported for doctor")
	case FormatTable:
		if _, err := fmt.Fprintf(
			w,
			"Version: %s\nCommit: %s\nDate: %s\nGo: %s\nOS/Arch: %s/%s\nConfig Dir: %s\nCache Dir: %s\nConfig File: %s\nSession File: %s\nLineage File: %s\n",
			report.Version.Version,
			report.Version.Commit,
			emptyFallback(report.Version.Date, "n/a"),
			report.GoVersion,
			report.OS,
			report.Arch,
			report.Paths.ConfigDir,
			report.Paths.CacheDir,
			report.Paths.ConfigFile,
			report.Paths.SessionFile,
			report.Paths.LineageFile,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\nGeneral Checks:"); err != nil {
			return err
		}
		for _, check := range report.Checks {
			if err := writeDoctorCheck(w, check); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\nAuth Checks:"); err != nil {
			return err
		}
		for _, check := range report.Auth.Checks {
			if err := writeDoctorCheck(w, check); err != nil {
				return err
			}
		}
		if report.OpenAPISummary != "" {
			if _, err := fmt.Fprintf(w, "\n%s\n", report.OpenAPISummary); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func WriteAuthDoctorReport(w io.Writer, format Format, report doctor.AuthReport) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case FormatCSV:
		return fmt.Errorf("csv output is not supported for auth doctor")
	case FormatTable:
		if _, err := fmt.Fprintf(w, "Python: %s\nHelper Dir: %s\n", report.PythonBinary, report.HelperDir); err != nil {
			return err
		}
		for _, check := range report.Checks {
			if err := writeDoctorCheck(w, check); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func writeDoctorCheck(w io.Writer, check doctor.Check) error {
	if _, err := fmt.Fprintf(w, "- [%s] %s: %s\n", check.Status, check.Name, check.Summary); err != nil {
		return err
	}
	if check.Detail != "" {
		if _, err := fmt.Fprintf(w, "  %s\n", check.Detail); err != nil {
			return err
		}
	}
	return nil
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
