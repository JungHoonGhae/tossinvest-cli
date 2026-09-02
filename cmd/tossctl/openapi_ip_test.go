package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
)

func TestOpenAPIIPCommandExposesSafeReplaceFlow(t *testing.T) {
	t.Parallel()
	cmd := newOpenAPIIPCmd(&rootOptions{})
	replace, _, err := cmd.Find([]string{"replace-current"})
	if err != nil {
		t.Fatalf("find replace-current: %v", err)
	}
	if replace.Flags().Lookup("execute") == nil || replace.Flags().Lookup("confirm") == nil {
		t.Fatal("replace-current must expose execute and confirm flags")
	}
	if replace.Annotations["source"] != "wts" {
		t.Fatalf("source = %q, want wts", replace.Annotations["source"])
	}
	if replace.Annotations["mutating"] != "" {
		t.Fatal("non-trading settings mutation must not use the live-trading mutating annotation")
	}
}

func TestRenderOpenAPIIPPlanJSON(t *testing.T) {
	t.Parallel()
	plan := openapiip.Plan{
		Kind:         "openapi_ip_replace_current",
		CurrentIP:    "198.51.100.9",
		ExistingIPs:  []string{"203.0.113.7"},
		DeleteIPs:    []string{"203.0.113.7"},
		AddIP:        "198.51.100.9",
		ConfirmToken: "abc123",
	}
	var buf bytes.Buffer
	if err := renderOpenAPIIPPlan(&buf, output.FormatJSON, plan); err != nil {
		t.Fatalf("render: %v", err)
	}
	var got openapiip.Plan
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ConfirmToken != plan.ConfirmToken || got.AddIP != plan.AddIP {
		t.Fatalf("unexpected JSON: %#v", got)
	}
}

func TestRenderOpenAPIIPListSupportsJSONTableAndEmpty(t *testing.T) {
	t.Parallel()
	var jsonOut bytes.Buffer
	if err := renderOpenAPIIPList(&jsonOut, output.FormatJSON, []string{"198.51.100.9", "203.0.113.7"}); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		AllowedIPs []string `json:"allowed_ips"`
	}
	if err := json.Unmarshal(jsonOut.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(payload.AllowedIPs, ","); got != "198.51.100.9,203.0.113.7" {
		t.Fatalf("allowed_ips = %q", got)
	}

	var tableOut bytes.Buffer
	if err := renderOpenAPIIPList(&tableOut, output.FormatTable, payload.AllowedIPs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tableOut.String(), "198.51.100.9, 203.0.113.7") {
		t.Fatalf("table = %q", tableOut.String())
	}

	var emptyOut bytes.Buffer
	if err := renderOpenAPIIPList(&emptyOut, output.FormatTable, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyOut.String(), "(none)") {
		t.Fatalf("empty table = %q", emptyOut.String())
	}

	var csvOut bytes.Buffer
	if err := renderOpenAPIIPList(&csvOut, output.FormatCSV, payload.AllowedIPs); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(csvOut.String())).ReadAll()
	if err != nil || len(records) != 3 || records[0][0] != "allowed_ip" || records[1][0] != "198.51.100.9" {
		t.Fatalf("list CSV records=%v err=%v", records, err)
	}
}

func TestRenderOpenAPIIPPlanCSV(t *testing.T) {
	t.Parallel()
	plan := openapiip.Plan{Kind: "openapi_ip_replace_current", CurrentIP: "198.51.100.9", ExistingIPs: []string{"203.0.113.7"}, DeleteIPs: []string{"203.0.113.7"}, AddIP: "198.51.100.9", ConfirmToken: "abc123"}
	var out bytes.Buffer
	if err := renderOpenAPIIPPlan(&out, output.FormatCSV, plan); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil || len(records) != 2 || len(records[0]) != len(records[1]) || records[0][0] != "kind" || records[1][1] != plan.CurrentIP {
		t.Fatalf("plan CSV records=%v err=%v", records, err)
	}
}

func TestRenderOpenAPIIPPlanTableShowsNextCommandOnlyForPreview(t *testing.T) {
	t.Parallel()
	plan := openapiip.Plan{
		Kind:         "openapi_ip_replace_current",
		CurrentIP:    "198.51.100.9",
		ExistingIPs:  []string{"203.0.113.7"},
		DeleteIPs:    []string{"203.0.113.7"},
		AddIP:        "198.51.100.9",
		ConfirmToken: "abc123",
	}
	var preview bytes.Buffer
	if err := renderOpenAPIIPPlan(&preview, output.FormatTable, plan); err != nil {
		t.Fatalf("render preview: %v", err)
	}
	if !strings.Contains(preview.String(), "--execute --confirm abc123") {
		t.Fatalf("preview lacks next command: %s", preview.String())
	}

	plan.Applied = true
	var applied bytes.Buffer
	if err := renderOpenAPIIPPlan(&applied, output.FormatTable, plan); err != nil {
		t.Fatalf("render applied: %v", err)
	}
	if strings.Contains(applied.String(), "--execute") {
		t.Fatalf("applied result must not suggest execution again: %s", applied.String())
	}
}

type openAPIIPFailingWriter struct {
	writes int
}

func (w *openAPIIPFailingWriter) Write(p []byte) (int, error) {
	w.writes++
	return 0, fmt.Errorf("boom")
}

func TestRenderOpenAPIIPPlanReportsTableWriteFailure(t *testing.T) {
	t.Parallel()
	plan := openapiip.Plan{CurrentIP: "198.51.100.9", AddIP: "198.51.100.9", ConfirmToken: "abc123"}
	if err := renderOpenAPIIPPlan(&openAPIIPFailingWriter{}, output.FormatTable, plan); err == nil {
		t.Fatal("late table write failure must be returned")
	}
}
