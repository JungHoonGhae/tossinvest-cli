package confirmation

import (
	"testing"
	"time"
)

func TestTokenAndMatches(t *testing.T) {
	t.Parallel()
	token := Token(`{"kind":"synthetic"}`)
	if len(token) != tokenLength || token != Token(`{"kind":"synthetic"}`) {
		t.Fatalf("token = %q", token)
	}
	if !Matches(token, token) || Matches("wrong", token) || Matches(" "+token, token) {
		t.Fatal("confirmation matching must be exact")
	}
}

func TestTimeBoundTokenIsKeyedExactAndExpires(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	token, err := IssueTimeBound([]byte("session-key-a"), "state", now, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyTimeBound(token, []byte("session-key-a"), "state", now) {
		t.Fatal("fresh token did not verify")
	}
	for _, changed := range []struct {
		key       string
		canonical string
		time      time.Time
	}{
		{key: "session-key-b", canonical: "state", time: now},
		{key: "session-key-a", canonical: "changed", time: now},
		{key: "session-key-a", canonical: "state", time: now.Add(5*time.Minute + time.Second)},
	} {
		if VerifyTimeBound(token, []byte(changed.key), changed.canonical, changed.time) {
			t.Fatalf("token verified with changed input: %+v", changed)
		}
	}
	if _, err := IssueTimeBound(nil, "state", now, time.Minute); err == nil {
		t.Fatal("missing key was accepted")
	}
}
