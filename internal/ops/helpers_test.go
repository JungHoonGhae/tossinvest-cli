package ops

import (
	"strings"
	"testing"
)

// probe check 헬퍼는 주간 모니터가 API 계약 변화를 잡는 근거다. 여기가 틀리면
// 모니터링이 조용히 눈을 감는다 — 통과해야 할 때 실패하는 것보다 나쁘다.

func TestExpectStatus(t *testing.T) {
	if err := ExpectStatus(200, 200); err != nil {
		t.Errorf("일치하는데 오류: %v", err)
	}
	err := ExpectStatus(503, 200)
	if err == nil {
		t.Fatal("불일치를 통과시켰다 — 모니터가 장애를 놓친다")
	}
	if !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "200") {
		t.Errorf("오류에 실제/기대 상태가 둘 다 있어야 한다: %v", err)
	}
}

func TestExpectPath(t *testing.T) {
	body := []byte(`{"result":{"totalAssetAmount":{"krw":1},"name":"x","items":[1],"flag":true,"nil":null}}`)
	ok := []struct{ path, typ string }{
		{"result", "object"},
		{"result.totalAssetAmount", "object"},
		{"result.totalAssetAmount.krw", "number"},
		{"result.name", "string"},
		{"result.items", "array"},
		{"result.flag", "bool"},
		{"result.nil", "null"},
	}
	for _, c := range ok {
		if err := ExpectPath(body, c.path, c.typ); err != nil {
			t.Errorf("ExpectPath(%q,%q) 실패: %v", c.path, c.typ, err)
		}
	}

	// 계약이 깨진 상황들 — 전부 잡아야 한다
	bad := []struct {
		name, path, typ string
		body            []byte
		wantIn          string
	}{
		{"키 사라짐", "result.gone", "number", body, "missing"},
		{"타입 바뀜", "result.name", "number", body, "expected number"},
		{"중간이 객체가 아님", "result.name.deeper", "string", body, "expected object"},
		{"JSON 아님", "result", "object", []byte("<html>503</html>"), "decode body"},
	}
	for _, c := range bad {
		err := ExpectPath(c.body, c.path, c.typ)
		if err == nil {
			t.Errorf("%s: 통과시켰다 — 모니터가 계약 변화를 놓친다", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantIn) {
			t.Errorf("%s: 오류에 %q 가 없다: %v", c.name, c.wantIn, err)
		}
	}
}

// 인자 강제변환은 48개 핸들러가 공유한다. 에이전트가 잘못된 타입을 보냈을 때
// 조용히 0/false 로 흘러가지 않고 분명한 오류가 나와야 한다.
func TestArgCoercion(t *testing.T) {
	args := map[string]any{
		"num": float64(5), "intish": 7,
		"flag": true, "text": "hi",
		"list": []any{"a", "b"},
	}

	t.Run("정상 변환", func(t *testing.T) {
		if v, err := argInt(args, "num"); err != nil || v != 5 {
			t.Errorf("argInt(num) = %v, %v", v, err)
		}
		if v, err := argInt(args, "intish"); err != nil || v != 7 {
			t.Errorf("argInt(intish) = %v, %v", v, err)
		}
		if v, err := argBool(args, "flag"); err != nil || !v {
			t.Errorf("argBool = %v, %v", v, err)
		}
		if v, err := argStringSlice(args, "list"); err != nil || len(v) != 2 || v[0] != "a" {
			t.Errorf("argStringSlice = %v, %v", v, err)
		}
	})

	t.Run("없는 키는 제로값 + 오류 없음", func(t *testing.T) {
		for _, f := range []func() error{
			func() error { _, e := argInt(args, "nope"); return e },
			func() error { _, e := argBool(args, "nope"); return e },
			func() error { _, e := argStringSlice(args, "nope"); return e },
			func() error { _, e := argString(args, "nope"); return e },
		} {
			if err := f(); err != nil {
				t.Errorf("선택 인자 부재는 오류가 아니어야 한다: %v", err)
			}
		}
	})

	t.Run("타입 불일치는 분명한 오류", func(t *testing.T) {
		cases := []struct {
			name string
			fn   func() error
		}{
			{"int 자리에 문자열", func() error { _, e := argInt(args, "text"); return e }},
			{"bool 자리에 문자열", func() error { _, e := argBool(args, "text"); return e }},
			{"배열 자리에 문자열", func() error { _, e := argStringSlice(args, "text"); return e }},
			{"문자열 자리에 bool", func() error { _, e := argString(args, "flag"); return e }},
		}
		for _, c := range cases {
			err := c.fn()
			if err == nil {
				t.Errorf("%s: 조용히 제로값으로 흘렀다", c.name)
				continue
			}
			if !strings.Contains(err.Error(), "must be") {
				t.Errorf("%s: 오류 문구가 불친절하다: %v", c.name, err)
			}
		}
		// 배열 안의 원소 타입도 검사해야 한다
		bad := map[string]any{"list": []any{"ok", 3}}
		if _, err := argStringSlice(bad, "list"); err == nil {
			t.Error("배열 원소 타입 불일치를 통과시켰다")
		}
	})
}

// Get 은 MCP describe_operation 의 근거다.
func TestCatalogGet(t *testing.T) {
	c := NewCatalog()
	op, ok := c.Get("accounts")
	if !ok {
		t.Fatal("알려진 오퍼레이션을 못 찾는다")
	}
	if op.ID != "accounts" {
		t.Errorf("ID = %q", op.ID)
	}
	if _, ok := c.Get("존재하지_않음"); ok {
		t.Error("없는 오퍼레이션을 찾았다고 한다")
	}
}
