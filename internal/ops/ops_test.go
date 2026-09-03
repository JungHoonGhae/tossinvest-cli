package ops

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strconv"
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
	if got, want := c.Count(), len(c.ops); got != want {
		t.Fatalf("Count() = %d, want complete registry size %d", got, want)
	}

	seenID := map[string]bool{}
	seenProbe := map[string]bool{}
	referencedProbe := map[string]bool{}
	for _, probe := range sharedWTSProbes() {
		if probe.Name == "" || probe.Check == nil {
			t.Errorf("shared probe must have Name and Check")
		}
		if seenProbe[probe.Name] {
			t.Errorf("duplicate shared probe name %q", probe.Name)
		}
		seenProbe[probe.Name] = true
	}
	for _, o := range all {
		if o.ID == "" || o.Summary == "" || o.Category == "" {
			t.Errorf("operation %+v: ID/Summary/Category must be set", o.ID)
		}
		if seenID[o.ID] {
			t.Errorf("duplicate operation id %q", o.ID)
		}
		seenID[o.ID] = true
		for _, alias := range o.Aliases {
			if alias == "" || alias == o.ID {
				t.Errorf("%s: invalid operation alias %q", o.ID, alias)
			}
			if seenID[alias] {
				t.Errorf("%s: duplicate operation id or alias %q", o.ID, alias)
			}
			seenID[alias] = true
			resolved, ok := c.Get(alias)
			if !ok || resolved.ID != o.ID {
				t.Errorf("%s: alias %q did not resolve to canonical operation", o.ID, alias)
			}
		}

		switch o.Backend {
		case "", "wts", "none", "auto":
		default:
			t.Errorf("%s: unknown backend %q", o.ID, o.Backend)
		}
		// WTS 전용 op 는 경로 관례(wts:)를 따른다 — 카탈로그 검색성 유지.
		if o.Backend == "wts" && !strings.HasPrefix(o.Path, "wts:") {
			t.Errorf("%s: wts operation path %q must start with \"wts:\"", o.ID, o.Path)
		}
		if o.Write {
			if o.Mutation == nil {
				t.Errorf("%s: write operation is missing mutation policy", o.ID)
			} else {
				if o.Mutation.RiskLevel == "" || o.Mutation.Reversibility == "" ||
					o.Mutation.AuthorizationMode == "" || o.Mutation.Verification == "" {
					t.Errorf("%s: incomplete mutation policy: %#v", o.ID, o.Mutation)
				}
				switch o.Mutation.AuthorizationMode {
				case MutationAuthorizationState:
					if !o.Mutation.RequiresPreview || !o.Mutation.RequiresFreshConfirmation {
						t.Errorf("%s: state-confirmation mutation must require preview and fresh state confirmation: %#v", o.ID, o.Mutation)
					}
				case MutationAuthorizationIntent:
					if !o.Mutation.RequiresPreview || o.Mutation.RequiresFreshConfirmation {
						t.Errorf("%s: intent-confirmation mutation must require preview without claiming state freshness: %#v", o.ID, o.Mutation)
					}
				case MutationAuthorizationMandate:
					if o.Mutation.RequiresFreshConfirmation || !o.Mutation.RequiresConfigOptIn {
						t.Errorf("%s: bounded-mandate mutation must use config opt-in instead of fresh confirmation: %#v", o.ID, o.Mutation)
					}
				default:
					t.Errorf("%s: unknown mutation authorization mode %q", o.ID, o.Mutation.AuthorizationMode)
				}
			}
		} else if o.Mutation != nil {
			t.Errorf("%s: read operation unexpectedly declares mutation policy", o.ID)
		}

		for _, p := range o.Params {
			switch p.Type {
			case "string", "integer", "number", "boolean", "string[]":
			default:
				t.Errorf("%s: param %s has unknown type %q", o.ID, p.Name, p.Type)
			}
		}

		var probes []ProbeSpec
		if o.Probe != nil {
			probes = append(probes, *o.Probe)
		}
		probes = append(probes, o.ExtraProbes...)
		for _, probe := range probes {
			if probe.Name == "" || probe.Check == nil {
				t.Errorf("%s: probe must have Name and Check", o.ID)
			}
			if seenProbe[probe.Name] {
				t.Errorf("%s: duplicate probe name %q", o.ID, probe.Name)
			}
			seenProbe[probe.Name] = true
			u, err := url.Parse(probe.URL)
			if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Host, ".tossinvest.com") {
				t.Errorf("%s: probe URL %q must be https on *.tossinvest.com", o.ID, probe.URL)
			}
			if probe.Method != "GET" && probe.Method != "POST" {
				t.Errorf("%s: probe method %q must be GET or POST", o.ID, probe.Method)
			}
		}
		for _, ref := range o.ProbeRefs {
			if !seenProbe[ref] {
				t.Errorf("%s: unknown shared probe reference %q", o.ID, ref)
			}
			referencedProbe[ref] = true
		}
	}
	for _, probe := range sharedWTSProbes() {
		if !referencedProbe[probe.Name] {
			t.Errorf("shared probe %q is not referenced by any operation", probe.Name)
		}
	}

	// Catalog.Probes() 는 선언된 probe 를 전부, 한 번씩 노출한다.
	if got := len(c.Probes()); got != len(seenProbe) {
		t.Errorf("Probes() returned %d specs, want %d", got, len(seenProbe))
	}
}

func TestLegacyBankingStatusAliasResolvesToSecuritiesFunding(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	canonical, ok := catalog.Get("accumulation_funding_status")
	if !ok {
		t.Fatal("accumulation_funding_status operation missing")
	}
	legacy, ok := catalog.Get("banking_status")
	if !ok || legacy.ID != canonical.ID {
		t.Fatalf("banking_status alias = %#v, want canonical %q", legacy, canonical.ID)
	}
	if canonical.Domain != "securities" || canonical.Category != "accumulate" {
		t.Fatalf("canonical funding classification = domain %q category %q", canonical.Domain, canonical.Category)
	}
	found := catalog.List("banking_status", 0)
	if len(found) != 1 || found[0].ID != canonical.ID || len(found[0].Aliases) != 1 || found[0].Aliases[0] != "banking_status" {
		t.Fatalf("alias search = %#v", found)
	}
}

func TestFinancialOperationsDeclareStrongestGuardrail(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{
		"place_order", "cancel_order", "modify_order",
		"place_conditional_order", "cancel_conditional_order", "modify_conditional_order",
	} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Fatalf("missing operation %q", id)
		}
		if op.Mutation == nil || op.Mutation.RiskLevel != MutationRiskFinancial ||
			op.Mutation.Reversibility != MutationIrreversible || !op.Mutation.RequiresConfigOptIn ||
			op.Mutation.AuthorizationMode != MutationAuthorizationIntent || op.Mutation.RequiresFreshConfirmation {
			t.Errorf("%s: weak financial mutation policy: %#v", id, op.Mutation)
		}
	}
}

func TestCatalogProbesIncludesOnlyReferencedSharedProbes(t *testing.T) {
	local := ProbeSpec{Name: "local", Method: "GET", URL: "https://wts-api.tossinvest.com/local", Check: statusAndPath("result", "object")}
	catalog := &Catalog{ops: []Operation{{ID: "example", Probe: &local}}}

	probes := catalog.Probes()
	if len(probes) != 1 || probes[0].Name != "local" {
		t.Fatalf("unreferenced shared probes leaked into catalog: %#v", probes)
	}
}

func TestBatchQuoteSummariesDescribeMissingInputs(t *testing.T) {
	catalog := NewCatalog()
	for _, id := range []string{"quote_charts", "quote_reasons"} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Fatalf("operation %q not found", id)
		}
		if !strings.Contains(op.Summary, "missing") || !strings.Contains(op.Summary, "requested order") {
			t.Errorf("%s summary does not describe its output contract: %q", id, op.Summary)
		}
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

func TestCatalogCallValidatesDeclaredArgumentTypesBeforeBackend(t *testing.T) {
	c := NewCatalog()

	_, err := c.Call(context.Background(), nil, "prices", map[string]any{
		"symbols": "AAPL",
	})
	if err == nil || !strings.Contains(err.Error(), `parameter "symbols" must be an array of strings`) {
		t.Fatalf("wrong string[] argument must fail before backend dispatch, got %v", err)
	}
}

func TestCatalogCallRejectsFractionalInteger(t *testing.T) {
	c := NewCatalog()

	_, err := c.Call(context.Background(), nil, "stock_ranking", map[string]any{
		"size": 1.5,
	})
	if err == nil || !strings.Contains(err.Error(), `parameter "size" must be an integer`) {
		t.Fatalf("fractional integer must not be truncated, got %v", err)
	}
}

func TestCatalogCallRejectsIntegerOutsideNativeRange(t *testing.T) {
	c := NewCatalog()
	tooLarge := math.Ldexp(1, strconv.IntSize-1)

	_, err := c.Call(context.Background(), nil, "stock_ranking", map[string]any{
		"size": tooLarge,
	})
	if err == nil || !strings.Contains(err.Error(), `parameter "size" must be an integer`) {
		t.Fatalf("out-of-range integer must fail before conversion, got %v", err)
	}
}

func TestCatalogCallRejectsLossyIntegerForNumber(t *testing.T) {
	if strconv.IntSize <= 53 {
		t.Skip("native int cannot exceed float64's exact integer range")
	}
	handlerCalled := false
	op := Operation{
		ID: "test_number", Backend: "none",
		Params: []Param{{Name: "amount", Type: "number", Required: true}},
		handler: func(_ context.Context, _ *Deps, args map[string]any) (any, error) {
			handlerCalled = true
			return args["amount"], nil
		},
	}
	c := &Catalog{ops: []Operation{op}, byID: map[string]Operation{op.ID: op}}

	got, err := c.Call(context.Background(), &Deps{}, op.ID, map[string]any{"amount": int(1 << 53)})
	if err != nil || got != float64(1<<53) {
		t.Fatalf("largest universally exact integer rejected: got=%v err=%v", got, err)
	}
	handlerCalled = false
	_, err = c.Call(context.Background(), &Deps{}, op.ID, map[string]any{"amount": int(1<<53) + 1})
	if err == nil || !strings.Contains(err.Error(), "precision loss") {
		t.Fatalf("lossy int-to-float64 input must be rejected, got %v", err)
	}
	if handlerCalled {
		t.Fatal("handler ran for a lossy numeric input")
	}
}

func TestCatalogCallRejectsLossyJSONNumber(t *testing.T) {
	handlerCalled := false
	op := Operation{
		ID: "test_json_number", Backend: "none",
		Params: []Param{{Name: "amount", Type: "number", Required: true}},
		handler: func(_ context.Context, _ *Deps, args map[string]any) (any, error) {
			handlerCalled = true
			return args["amount"], nil
		},
	}
	c := &Catalog{ops: []Operation{op}, byID: map[string]Operation{op.ID: op}}

	_, err := c.Call(context.Background(), &Deps{}, op.ID, map[string]any{
		"amount": json.Number("9007199254740993"),
	})
	if err == nil || !strings.Contains(err.Error(), "precision loss") {
		t.Fatalf("lossy JSON number must be rejected before dispatch, got %v", err)
	}
	if handlerCalled {
		t.Fatal("handler ran for a lossy JSON numeric input")
	}
}

func TestCatalogCallRejectsInvalidPrimitiveShapes(t *testing.T) {
	tests := []struct {
		name  string
		param Param
		value any
		want  string
	}{
		{name: "string", param: Param{Name: "value", Type: "string"}, value: 1, want: "must be a string"},
		{name: "boolean", param: Param{Name: "value", Type: "boolean"}, value: "true", want: "must be a boolean"},
		{name: "array member", param: Param{Name: "value", Type: "string[]"}, value: []any{"ok", 1}, want: `parameter "value"[1] must be a string`},
		{name: "NaN number", param: Param{Name: "value", Type: "number"}, value: math.NaN(), want: "must be a number"},
		{name: "non-finite number", param: Param{Name: "value", Type: "number"}, value: math.Inf(1), want: "must be a number"},
		{name: "unsupported declaration", param: Param{Name: "value", Type: "object"}, value: map[string]any{}, want: "unsupported declared type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := Operation{
				ID: "test_shape", Backend: "none", Params: []Param{tt.param},
				handler: func(context.Context, *Deps, map[string]any) (any, error) {
					t.Fatal("handler must not run for an invalid primitive shape")
					return nil, nil
				},
			}
			c := &Catalog{ops: []Operation{op}, byID: map[string]Operation{op.ID: op}}
			_, err := c.Call(context.Background(), &Deps{}, op.ID, map[string]any{"value": tt.value})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestCatalogCallAcceptsDirectStringSlice(t *testing.T) {
	op := Operation{
		ID: "test_direct_strings", Backend: "none",
		Params: []Param{{Name: "symbols", Type: "string[]", Required: true}},
		handler: func(_ context.Context, _ *Deps, args map[string]any) (any, error) {
			return args["symbols"], nil
		},
	}
	c := &Catalog{ops: []Operation{op}, byID: map[string]Operation{op.ID: op}}
	want := []string{"005930", "AAPL"}
	got, err := c.Call(context.Background(), &Deps{}, op.ID, map[string]any{"symbols": want})
	if err != nil {
		t.Fatal(err)
	}
	values, ok := got.([]string)
	if !ok || len(values) != len(want) || values[0] != want[0] || values[1] != want[1] {
		t.Fatalf("direct []string changed during normalization: %#v", got)
	}
}

func TestCatalogCallReportsUnknownArgumentsDeterministically(t *testing.T) {
	c := NewCatalog()
	_, err := c.Call(context.Background(), &Deps{}, "auth_status", map[string]any{"zeta": true, "alpha": true})
	if err == nil || !strings.Contains(err.Error(), `unknown parameter "alpha"`) {
		t.Fatalf("unknown arguments must be sorted before reporting, got %v", err)
	}
}

func TestCatalogCallRejectsUnknownArgument(t *testing.T) {
	c := NewCatalog()

	_, err := c.Call(context.Background(), &Deps{}, "auth_status", map[string]any{
		"typo": true,
	})
	if err == nil || !strings.Contains(err.Error(), `operation "auth_status" has unknown parameter "typo"`) {
		t.Fatalf("unknown argument must be rejected, got %v", err)
	}
}

func TestCatalogCallNormalizesJSONStringArrayForHandler(t *testing.T) {
	op := Operation{
		ID: "test_strings", Backend: "none",
		Params: []Param{{Name: "symbols", Type: "string[]", Required: true}},
		handler: func(_ context.Context, _ *Deps, args map[string]any) (any, error) {
			return argStringSlice(args, "symbols")
		},
	}
	c := &Catalog{ops: []Operation{op}, byID: map[string]Operation{op.ID: op}}

	got, err := c.Call(context.Background(), &Deps{}, op.ID, map[string]any{
		"symbols": []any{"005930", "AAPL"},
	})
	if err != nil {
		t.Fatalf("valid JSON string array rejected: %v", err)
	}
	symbols, ok := got.([]string)
	if !ok || len(symbols) != 2 || symbols[0] != "005930" || symbols[1] != "AAPL" {
		t.Fatalf("handler received wrong normalized value: %#v", got)
	}
}

func TestCatalogCallRejectsMissingDependenciesWithoutPanic(t *testing.T) {
	c := NewCatalog()

	_, err := c.Call(context.Background(), nil, "auth_status", nil)
	if err == nil || !strings.Contains(err.Error(), "dependencies are not configured") {
		t.Fatalf("nil dependencies must return a configuration error, got %v", err)
	}
}
