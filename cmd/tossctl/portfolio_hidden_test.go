package main

import "testing"

func TestPortfolioHiddenCommandContract(t *testing.T) {
	portfolio := newPortfolioCmd(&rootOptions{})
	for _, path := range [][]string{{"hidden", "list"}, {"hidden", "hide"}, {"hidden", "show"}} {
		cmd, _, err := portfolio.Find(path)
		if err != nil || cmd == portfolio || cmd.Name() != path[len(path)-1] {
			t.Fatalf("%v command not registered: cmd=%q err=%v", path, cmd.Name(), err)
		}
		if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" {
			t.Fatalf("%v annotations = %#v", path, cmd.Annotations)
		}
	}
}
