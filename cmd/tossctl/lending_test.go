package main

import "testing"

func TestLendingTopCommandContract(t *testing.T) {
	t.Parallel()
	root := newLendingCmd(&rootOptions{})
	cmd, _, err := root.Find([]string{"top"})
	if err != nil || cmd == root {
		t.Fatalf("top command missing: cmd=%q err=%v", cmd.Name(), err)
	}
	if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
}
