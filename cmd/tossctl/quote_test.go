package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestQuoteMetadataCommandContract(t *testing.T) {
	quote := newQuoteCmd(&rootOptions{})
	cmd, _, err := quote.Find([]string{"metadata"})
	if err != nil || cmd == quote || cmd.Name() != "metadata" {
		t.Fatalf("metadata command not registered: cmd=%q err=%v", cmd.Name(), err)
	}
	if cmd.Annotations["source"] != "official" {
		t.Fatalf("source = %q, want official", cmd.Annotations["source"])
	}
	if !strings.Contains(cmd.Use, "<symbol>") {
		t.Fatalf("Use = %q, want symbol contract", cmd.Use)
	}
	if err := cmd.Args(cmd, nil); err == nil {
		t.Fatal("metadata must reject an empty symbol list")
	}
	if err := cmd.Args(cmd, []string{"AAPL,005930"}); err != nil {
		t.Fatalf("metadata rejected valid symbols: %v", err)
	}
}

func TestQuoteAlertCommandContract(t *testing.T) {
	quote := newQuoteCmd(&rootOptions{})
	for _, path := range [][]string{{"alert", "list"}, {"alert", "add"}, {"alert", "remove"}} {
		cmd, _, err := quote.Find(path)
		if err != nil || cmd == quote || cmd.Name() != path[len(path)-1] {
			t.Fatalf("%v command not registered: cmd=%q err=%v", path, cmd.Name(), err)
		}
		if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" {
			t.Fatalf("%v annotations = %#v", path, cmd.Annotations)
		}
	}
}

func TestParseBatchSymbols(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "single arg",
			args: []string{"삼성전자"},
			want: []string{"삼성전자"},
		},
		{
			name: "space-separated args",
			args: []string{"삼성전자", "KB금융"},
			want: []string{"삼성전자", "KB금융"},
		},
		{
			name: "comma-separated single arg",
			args: []string{"삼성전자,KB금융"},
			want: []string{"삼성전자", "KB금융"},
		},
		{
			name: "mixed comma + space",
			args: []string{"삼성전자,KB금융", "현대차"},
			want: []string{"삼성전자", "KB금융", "현대차"},
		},
		{
			name: "trims whitespace around commas",
			args: []string{"삼성전자 , KB금융 ,  현대차"},
			want: []string{"삼성전자", "KB금융", "현대차"},
		},
		{
			name: "drops empty tokens",
			args: []string{"삼성전자,,KB금융,"},
			want: []string{"삼성전자", "KB금융"},
		},
		{
			name: "all-empty stays empty",
			args: []string{",", " , ", ""},
			want: nil,
		},
	}
	for _, c := range cases {
		got := parseBatchSymbols(c.args)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: parseBatchSymbols(%#v) = %#v, want %#v", c.name, c.args, got, c.want)
		}
	}
}
