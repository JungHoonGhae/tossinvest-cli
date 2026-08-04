package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func TestGetPopularBoards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/boards/popular-follower" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"results": []map[string]any{
					{"id": 1, "subjectType": "LOUNGE", "subjectId": "LOUNGE_1", "title": "가장인기있는라운지",
						"about": "설명", "rules": []string{"규칙1", "규칙2"},
						"followerCount": 900, "commentCount": 30, "isMember": true, "isManager": false,
						"createdAt": "2024-01-01"},
					{"id": 2, "subjectType": "LOUNGE", "subjectId": "LOUNGE_2", "title": "두번째라운지",
						"followerCount": 100, "commentCount": 5, "isMember": false, "isManager": false},
				},
			},
		})
	}))
	defer server.Close()

	got, err := testClientFor(server).GetPopularBoards(t.Context())
	if err != nil {
		t.Fatalf("GetPopularBoards() error = %v", err)
	}
	if len(got.Boards) != 2 {
		t.Fatalf("boards = %d, want 2", len(got.Boards))
	}
	// 서버 순서가 곧 랭킹이다. 재정렬하면 앱과 다른 순위를 보여주게 된다.
	if got.Boards[0].Title != "가장인기있는라운지" || got.Boards[1].Title != "두번째라운지" {
		t.Errorf("server order not preserved: %s, %s", got.Boards[0].Title, got.Boards[1].Title)
	}
	if got.Boards[0].FollowerCount != 900 || !got.Boards[0].IsMember {
		t.Errorf("board[0] = %+v, want 900 followers and member=true", got.Boards[0])
	}
	if len(got.Boards[0].Rules) != 2 {
		t.Errorf("rules = %v, want 2 entries", got.Boards[0].Rules)
	}
}
