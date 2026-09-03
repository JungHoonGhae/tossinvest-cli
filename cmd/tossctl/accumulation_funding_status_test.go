package main

import "testing"

func TestAccumulationFundingStatusIsCanonicalAndBankingStatusIsDeprecated(t *testing.T) {
	canonical, _, err := newRootCmd().Find([]string{"accumulate", "funding-status"})
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Annotations["domain"] != "securities" || canonical.Annotations["source"] != "wts" {
		t.Fatalf("canonical annotations = %#v", canonical.Annotations)
	}

	legacy, _, err := newRootCmd().Find([]string{"banking", "status"})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Deprecated == "" {
		t.Fatal("legacy banking status command must direct users to the canonical Securities command")
	}
}
