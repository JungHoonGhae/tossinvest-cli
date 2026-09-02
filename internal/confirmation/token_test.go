package confirmation

import "testing"

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
