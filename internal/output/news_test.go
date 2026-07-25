package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func sampleNews() domain.MarketNews {
	return domain.MarketNews{
		Type:  "PERSONALIZE_WATCH",
		Title: "관심 뉴스",
		Items: []domain.NewsItem{{
			ID: "n1", Title: "더미 헤드라인", Summary: "더미 요약문",
			Source: "더미통신", CreatedAt: "2026-07-25T09:00:00",
			Stocks: []domain.NewsRelatedStock{
				{Code: "A000000", Name: "더미종목", Fluctuation: 2.15},
				{Code: "A111111", Name: "더미종목2", Fluctuation: -0.4},
			},
		}},
		FetchedAt: time.Now(),
	}
}

// 관련 종목 등락률은 이 피드가 헤드라인 목록보다 나은 유일한 지점이다 — 항상 나와야 한다.
func TestMarketNewsAlwaysShowsRelatedStockMoves(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMarketNews(&buf, FormatTable, sampleNews(), false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"더미종목 +2.15%", "더미종목2 -0.40%"} {
		if !strings.Contains(out, want) {
			t.Errorf("관련 종목 등락률 %q 가 없다:\n%s", want, out)
		}
	}
}

// 요약은 기본 생략(50건이면 벽), --full 로만.
func TestMarketNewsSummaryIsOptIn(t *testing.T) {
	n := sampleNews()
	var plain, full bytes.Buffer
	if err := WriteMarketNews(&plain, FormatTable, n, false); err != nil {
		t.Fatal(err)
	}
	if err := WriteMarketNews(&full, FormatTable, n, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "더미 요약문") {
		t.Error("기본 출력에 요약이 들어갔다")
	}
	if !strings.Contains(plain.String(), "--full") {
		t.Error("요약이 있는데 --full 안내가 없다")
	}
	if !strings.Contains(full.String(), "더미 요약문") {
		t.Error("--full 인데 요약이 없다")
	}
}

// 범위 라벨은 서버가 준 title 을 쓴다 — 한글 라벨을 코드에 박지 않기 위함.
func TestMarketNewsUsesServerTitle(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMarketNews(&buf, FormatTable, sampleNews(), false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "관심 뉴스") {
		t.Error("서버가 준 title 이 헤더에 없다")
	}

	// title 이 없으면 enum 으로 폴백
	n := sampleNews()
	n.Title = ""
	var buf2 bytes.Buffer
	if err := WriteMarketNews(&buf2, FormatTable, n, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf2.String(), "PERSONALIZE_WATCH") {
		t.Error("title 이 없을 때 enum 폴백이 없다")
	}
}

func TestMarketNewsCSVIsFlat(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMarketNews(&buf, FormatCSV, sampleNews(), false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("헤더 + 1행이어야 한다: %v", lines)
	}
	// 관련 종목은 한 셀에 기계가 나눌 수 있는 형태로
	if !strings.Contains(lines[1], "더미종목:A000000:2.15|") {
		t.Errorf("CSV 의 stocks 셀 형식이 다르다: %s", lines[1])
	}
}

func TestMarketNewsEmpty(t *testing.T) {
	var buf bytes.Buffer
	n := sampleNews()
	n.Items = nil
	if err := WriteMarketNews(&buf, FormatTable, n, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "없습니다") {
		t.Errorf("빈 결과 안내가 없다: %s", buf.String())
	}
}
