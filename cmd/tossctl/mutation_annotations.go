package main

// mutationAnnotations is the CLI-side mirror of ops.MutationPolicy. Keeping
// these keys uniform lets shell agents distinguish a state change from a read
// without treating every non-GET action as a financial trade.
func mutationAnnotations(source, domain, risk, reversibility string) map[string]string {
	annotations := map[string]string{
		"source": source, "domain": domain, "writes_state": "true",
		"mutation_risk": risk, "reversibility": reversibility,
	}
	if risk == "financial" {
		// Backward-compatible marker consumed by existing order safety tooling.
		annotations["mutating"] = "true"
	}
	return annotations
}
