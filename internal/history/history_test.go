package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type fakeReader struct {
	owner         string
	key           []byte
	positions     []domain.Position
	pages         []domain.TransactionPage
	requests      int
	positionReads int
	fail          bool
}

func (f *fakeReader) ListAccounts(context.Context) ([]domain.Account, string, error) {
	return []domain.Account{{ID: f.owner}}, f.owner, nil
}
func (f *fakeReader) ConfirmationKey(string) []byte { return f.key }
func (f *fakeReader) ListPositions(context.Context) ([]domain.Position, error) {
	f.positionReads++
	return f.positions, nil
}
func (f *fakeReader) ListTransactions(_ context.Context, market string, _, _ time.Time, _ string, _, _ int) (domain.TransactionPage, error) {
	i := f.requests
	f.requests++
	if f.fail {
		return domain.TransactionPage{}, errors.New("private upstream diagnostic")
	}
	if i >= len(f.pages) {
		return domain.TransactionPage{LastPage: true, Items: []domain.Transaction{}}, nil
	}
	return f.pages[i], nil
}
func testService(t *testing.T) (*Service, *fakeReader) {
	t.Helper()
	f := &fakeReader{owner: "account-one", key: []byte("test-session"), positions: []domain.Position{{ProductCode: "A005930", Symbol: "005930", Name: "삼성전자", MarketType: "KR_STOCK", Quantity: 2, MarketValue: 120000, UnrealizedPnL: 10000}}}
	s := New(filepath.Join(t.TempDir(), "private", "history.sqlite"), f)
	s.now = func() time.Time { return time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC) }
	return s, f
}
func syncOnce(t *testing.T, s *Service, o SyncOptions) SyncResult {
	t.Helper()
	ctx := context.Background()
	preview, err := s.Sync(ctx, o, false, "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Sync(ctx, o, true, preview.ConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestStoreUsesExactPathWithURICharacters(t *testing.T) {
	s, _ := testService(t)
	s.Store.Path = filepath.Join(t.TempDir(), "saved # 100%", "history with spaces.sqlite")
	syncOnce(t, s, SyncOptions{Market: "kr"})
	if _, err := os.Stat(s.Store.Path); err != nil {
		t.Fatalf("database was not created at the requested path: %v", err)
	}
	items, err := s.Store.List(context.Background(), 20)
	if err != nil || len(items) != 1 {
		t.Fatalf("read database at exact path: %+v %v", items, err)
	}
}

func TestPreviewThenRoundTripAndCompareOffline(t *testing.T) {
	s, f := testService(t)
	ctx := context.Background()
	o := SyncOptions{From: "2026-09-01", To: "2026-09-11", Market: "kr"}
	f.pages = []domain.TransactionPage{{LastPage: true, Items: []domain.Transaction{{SortKey: "tx1", Date: "2026-09-10", Market: "kr", Currency: "KRW", Category: "trade", StockCode: "A005930", StockName: "삼성전자", Quantity: 2, Amount: 120000, Raw: []byte(`{"session":"never persist"}`)}}}}
	preview, err := s.Sync(ctx, o, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Store.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview created a database: %v", err)
	}
	if f.requests != 0 || f.positionReads != 0 || preview.ConfirmToken == "" {
		t.Fatal("preview fetched market data or omitted confirmation")
	}
	one, err := s.Sync(ctx, o, true, preview.ConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	if one.Snapshot.ID != 1 || !one.Snapshot.Complete || one.Snapshot.TransactionCount != 1 {
		t.Fatalf("receipt: %+v", one)
	}
	if _, err = s.Sync(ctx, o, true, preview.ConfirmToken); err == nil {
		t.Fatal("replayed preview succeeded")
	}
	f.positions = []domain.Position{{ProductCode: "A005930", Symbol: "005930", Name: "삼성전자", MarketType: "KR_STOCK", Quantity: 3, MarketValue: 190000, UnrealizedPnL: 12000}, {ProductCode: "US-AAPL", Symbol: "AAPL", Quantity: 0.5, MarketValue: 100000, MarketValueUSD: 75}}
	syncOnce(t, s, o)
	s.Reader = nil // subsequent operations prove that no API/credential is needed
	list, err := s.Store.List(ctx, 20)
	if err != nil || len(list) != 2 || list[0].ID != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}
	positions, err := s.Store.Positions(ctx, 1)
	if err != nil || positions.Positions[0].Quantity != 2 {
		t.Fatalf("positions: %+v %v", positions, err)
	}
	transactions, err := s.Store.Transactions(ctx, Query{SnapshotID: 1, Search: "삼성", Limit: 50})
	if err != nil || len(transactions.Items) != 1 || transactions.Items[0].Currency != "KRW" {
		t.Fatalf("search: %+v %v", transactions, err)
	}
	diff, err := s.Store.Compare(ctx, 1, 2)
	if err != nil || len(diff.Changes) != 2 {
		t.Fatalf("compare: %+v %v", diff, err)
	}
	var change Change
	for _, c := range diff.Changes {
		if c.Symbol == "005930" {
			change = c
		}
	}
	if change.QuantityDelta != 1 || change.ValueDeltaKRW != 70000 || change.PnLDeltaKRW != 2000 {
		t.Fatalf("change: %+v", change)
	}
	info, err := os.Stat(s.Store.Path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode: %v %v", info, err)
	}
	bytes, err := os.ReadFile(s.Store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bytes), "never persist") || strings.Contains(string(bytes), "account-one") {
		t.Fatal("raw account or payload was persisted")
	}
}

func TestSyncBindsIntentAccountSessionAndExpiry(t *testing.T) {
	for _, kind := range []string{"intent", "account", "session", "expiry"} {
		t.Run(kind, func(t *testing.T) {
			s, f := testService(t)
			o := SyncOptions{Market: "kr"}
			preview, err := s.Sync(context.Background(), o, false, "")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "intent":
				o.Market = "us"
			case "account":
				f.owner = "other"
			case "session":
				f.key = []byte("other-session")
			case "expiry":
				old := s.now()
				s.now = func() time.Time { return old.Add(6 * time.Minute) }
			}
			if _, err = s.Sync(context.Background(), o, true, preview.ConfirmToken); err == nil {
				t.Fatal("changed confirmation accepted")
			}
			if f.positionReads != 0 || f.requests != 0 {
				t.Fatal("unconfirmed execution read data")
			}
		})
	}
	s, f := testService(t)
	syncOnce(t, s, SyncOptions{Market: "kr"})
	f.owner = "other"
	if _, err := s.Sync(context.Background(), SyncOptions{}, false, ""); err == nil {
		t.Fatal("different account appended to existing history")
	}
}

func TestPagingCoverageAndLiteralSearch(t *testing.T) {
	s, f := testService(t)
	f.pages = []domain.TransactionPage{
		{Items: []domain.Transaction{{SortKey: "new", Date: "2026-09-10", StockName: "A%_", Currency: "KRW", Amount: 10}, {SortKey: "boundary", Date: "2026-09-09", Currency: "USD", Amount: 1}}},
		{Items: []domain.Transaction{{SortKey: "boundary", Date: "2026-09-09", Currency: "USD", Amount: 1}}},
	}
	r := syncOnce(t, s, SyncOptions{Market: "kr"})
	if r.Snapshot.Complete || r.Snapshot.TransactionCount != 2 || !strings.Contains(r.Snapshot.Warnings[0], "stopped advancing") {
		t.Fatalf("partial: %+v", r.Snapshot)
	}
	q := Query{Limit: 1}
	first, err := s.Store.Transactions(context.Background(), q)
	if err != nil || !first.HasMore || first.NextOffset != 1 {
		t.Fatalf("first: %+v %v", first, err)
	}
	q.Offset = first.NextOffset
	second, err := s.Store.Transactions(context.Background(), q)
	if err != nil || second.HasMore || second.Items[0].Currency != "USD" {
		t.Fatalf("second: %+v %v", second, err)
	}
	for search, want := range map[string]int{"%_": 1, "' OR 1=1 --": 0} {
		data, err := s.Store.Transactions(context.Background(), Query{Search: search, Limit: 50})
		if err != nil || len(data.Items) != want {
			t.Fatalf("literal search %q: %+v %v", search, data, err)
		}
	}
}

func TestFailureAndLimitsAreReportedWithoutReplacingEarlierCollections(t *testing.T) {
	s, f := testService(t)
	syncOnce(t, s, SyncOptions{Market: "kr"})
	f.fail = true
	r := syncOnce(t, s, SyncOptions{Market: "kr"})
	if r.Snapshot.Complete || len(r.Snapshot.Warnings) != 1 || strings.Contains(r.Snapshot.Warnings[0], "private") {
		t.Fatalf("failure receipt: %+v", r)
	}
	f.fail = false
	f.requests = 0
	f.pages = []domain.TransactionPage{{Items: []domain.Transaction{{SortKey: "one", Date: "2026-09-10"}}}}
	r = syncOnce(t, s, SyncOptions{Market: "kr", PageLimit: 1})
	if r.Snapshot.Complete || !strings.Contains(r.Snapshot.Warnings[0], "page limit") {
		t.Fatalf("page limit: %+v", r)
	}
	for _, o := range []SyncOptions{{From: "2026-01-01", To: "2026-09-11"}, {From: "bad"}, {PageLimit: -1}, {Market: "jp"}, {From: "2026-09-12", To: "2026-09-11"}} {
		if _, err := s.Sync(context.Background(), o, false, ""); err == nil {
			t.Fatalf("invalid options accepted: %+v", o)
		}
	}
	list, err := s.Store.List(context.Background(), 20)
	if err != nil || len(list) != 3 {
		t.Fatalf("prior collections lost: %+v %v", list, err)
	}
}

func TestConcurrentCommitsRejectStaleRevision(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	data := Data{Snapshot: Snapshot{CollectedAt: "2026-09-11T00:00:00Z", Markets: []string{"kr"}, Warnings: []string{}}, Positions: []domain.Position{}, Transactions: []Transaction{}}
	if _, err := s.Store.save(ctx, "owner", 0, data); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ch := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Store.save(ctx, "owner", 1, data); ch <- err }()
	}
	wg.Wait()
	close(ch)
	passes := 0
	for err := range ch {
		if err == nil {
			passes++
		} else if !strings.Contains(err.Error(), "changed since preview") {
			t.Fatal(err)
		}
	}
	if passes != 1 {
		t.Fatalf("commits=%d, want exactly one", passes)
	}
}

func TestInterruptedInitializationCanBeRetried(t *testing.T) {
	s, _ := testService(t)
	if err := os.MkdirAll(filepath.Dir(s.Store.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Store.Path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if list, err := s.Store.List(context.Background(), 20); err != nil || len(list) != 0 {
		t.Fatalf("empty store: %+v %v", list, err)
	}
	result := syncOnce(t, s, SyncOptions{Market: "kr"})
	if result.Snapshot.ID != 1 {
		t.Fatalf("retry: %+v", result)
	}
}
