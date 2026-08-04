package ops

import (
	"net/url"
	"strings"
	"testing"
)

// 레지스트리 불변식 — 오퍼레이션을 추가할 때 이 테스트 하나가 형식 실수를 잡는다.
func TestRegistryInvariants(t *testing.T) {
	c := NewCatalog()
	all := c.List("", 0)
	if len(all) == 0 {
		t.Fatal("empty catalog")
	}

	seenID := map[string]bool{}
	seenProbe := map[string]bool{}
	for _, o := range all {
		if o.ID == "" || o.Summary == "" || o.Category == "" {
			t.Errorf("operation %+v: ID/Summary/Category must be set", o.ID)
		}
		if seenID[o.ID] {
			t.Errorf("duplicate operation id %q", o.ID)
		}
		seenID[o.ID] = true

		switch o.Backend {
		case "", "wts", "none", "auto":
		default:
			t.Errorf("%s: unknown backend %q", o.ID, o.Backend)
		}
		// WTS 전용 op 는 경로 관례(wts:)를 따른다 — 카탈로그 검색성 유지.
		if o.Backend == "wts" && !strings.HasPrefix(o.Path, "wts:") {
			t.Errorf("%s: wts operation path %q must start with \"wts:\"", o.ID, o.Path)
		}

		for _, p := range o.Params {
			switch p.Type {
			case "string", "integer", "number", "boolean", "string[]":
			default:
				t.Errorf("%s: param %s has unknown type %q", o.ID, p.Name, p.Type)
			}
		}

		if o.Probe != nil {
			if o.Probe.Name == "" || o.Probe.Check == nil {
				t.Errorf("%s: probe must have Name and Check", o.ID)
			}
			if seenProbe[o.Probe.Name] {
				t.Errorf("%s: duplicate probe name %q", o.ID, o.Probe.Name)
			}
			seenProbe[o.Probe.Name] = true
			u, err := url.Parse(o.Probe.URL)
			if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Host, ".tossinvest.com") {
				t.Errorf("%s: probe URL %q must be https on *.tossinvest.com", o.ID, o.Probe.URL)
			}
			if o.Probe.Method != "GET" && o.Probe.Method != "POST" {
				t.Errorf("%s: probe method %q must be GET or POST", o.ID, o.Probe.Method)
			}
		}
	}

	// Catalog.Probes() 는 선언된 probe 를 전부, 한 번씩 노출한다.
	if got := len(c.Probes()); got != len(seenProbe) {
		t.Errorf("Probes() returned %d specs, want %d", got, len(seenProbe))
	}
}

func TestExpectPathArrayIndex(t *testing.T) {
	body := []byte(`{"result":[{"close":100},{"close":200}]}`)

	if err := ExpectPath(body, "result.0.close", "number"); err != nil {
		t.Errorf("result.0.close: %v", err)
	}
	if err := ExpectPath(body, "result.1.close", "number"); err != nil {
		t.Errorf("result.1.close: %v", err)
	}
	// An empty list must fail — that is the whole point of indexing rather than
	// stopping at "result is an array". A wrong product code returns [].
	if err := ExpectPath([]byte(`{"result":[]}`), "result.0.close", "number"); err == nil {
		t.Error("empty array: want error, got nil")
	}
	// Indexing a non-array must say so rather than reporting a missing key.
	if err := ExpectPath([]byte(`{"result":{"close":1}}`), "result.0.close", "number"); err == nil {
		t.Error("object indexed as array: want error, got nil")
	}
}
