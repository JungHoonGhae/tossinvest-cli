package watchlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type fakeClient struct {
	groups      []domain.WatchlistGroup
	items       map[int64][]domain.WatchlistItem
	mutations   int
	nextGroupID int64
	mutationErr error
	listCalls   int
	listErrAt   int
	itemCalls   int
	itemErrAt   int
	key         []byte
}

func (f *fakeClient) ListWatchlistGroups(context.Context) ([]domain.WatchlistGroup, error) {
	f.listCalls++
	if f.listErrAt > 0 && f.listCalls >= f.listErrAt {
		return nil, errors.New("synthetic group read failure")
	}
	return append([]domain.WatchlistGroup(nil), f.groups...), nil
}
func (f *fakeClient) GetWatchlistGroupItems(_ context.Context, id int64) ([]domain.WatchlistItem, error) {
	group, err := f.GetWatchlistGroup(context.Background(), id)
	return group.Items, err
}
func (f *fakeClient) GetWatchlistGroup(_ context.Context, id int64) (domain.WatchlistGroup, error) {
	f.itemCalls++
	if f.itemErrAt > 0 && f.itemCalls >= f.itemErrAt {
		return domain.WatchlistGroup{}, errors.New("synthetic item read failure")
	}
	for _, group := range f.groups {
		if group.ID == id {
			group.Items = append([]domain.WatchlistItem(nil), f.items[id]...)
			group.ItemCount = len(group.Items)
			return group, nil
		}
	}
	return domain.WatchlistGroup{}, fmt.Errorf("watchlist folder %d not found", id)
}
func (f *fakeClient) ConfirmationKey(string) []byte { return append([]byte(nil), f.key...) }
func (f *fakeClient) ResolveProductCode(context.Context, string) (string, error) {
	return "US.AAPL", nil
}
func (f *fakeClient) CreateWatchlistGroup(_ context.Context, name string) (domain.WatchlistGroup, error) {
	f.mutations++
	f.nextGroupID++
	g := domain.WatchlistGroup{ID: f.nextGroupID, Name: name, Type: "USER_MADE"}
	f.groups = append(f.groups, g)
	return g, f.mutationErr
}
func (f *fakeClient) RenameWatchlistGroup(_ context.Context, id int64, name string) error {
	f.mutations++
	for i := range f.groups {
		if f.groups[i].ID == id {
			f.groups[i].Name = name
		}
	}
	return f.mutationErr
}
func (f *fakeClient) DeleteWatchlistGroup(_ context.Context, id int64) error {
	f.mutations++
	for i := range f.groups {
		if f.groups[i].ID == id {
			f.groups = append(f.groups[:i], f.groups[i+1:]...)
			break
		}
	}
	delete(f.items, id)
	return f.mutationErr
}
func (f *fakeClient) AddWatchlistItem(_ context.Context, id int64, _ string) error {
	f.mutations++
	f.items[id] = append(f.items[id], domain.WatchlistItem{ProductCode: "US.AAPL", Symbol: "AAPL"})
	return f.mutationErr
}
func (f *fakeClient) RemoveWatchlistItem(_ context.Context, id int64, _ string) error {
	f.mutations++
	var kept []domain.WatchlistItem
	for _, item := range f.items[id] {
		if item.ProductCode != "US.AAPL" {
			kept = append(kept, item)
		}
	}
	f.items[id] = kept
	return f.mutationErr
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		groups:      []domain.WatchlistGroup{{ID: 7, Name: "Long term", Type: "USER_MADE", ItemCount: 1}},
		items:       map[int64][]domain.WatchlistItem{7: {{ProductCode: "A005930", Symbol: "005930"}}},
		nextGroupID: 7,
		key:         []byte("test-watchlist-confirmation-key"),
	}
}

func TestGroupChangeRequiresFreshConfirmationAndVerifies(t *testing.T) {
	t.Parallel()
	f := newFakeClient()
	s := NewService(f)
	preview, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ConfirmToken == "" || preview.Applied || preview.Noop || f.mutations != 0 {
		t.Fatalf("preview=%#v mutations=%d", preview, f.mutations)
	}
	if _, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{Execute: true, Confirm: "wrong"}); err == nil {
		t.Fatal("wrong confirmation was accepted")
	}
	result, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || f.groups[0].Name != "Retirement" || f.mutations != 1 {
		t.Fatalf("result=%#v groups=%#v mutations=%d", result, f.groups, f.mutations)
	}
}

func TestGroupDeleteRequiresExplicitIrreversibleAcknowledgement(t *testing.T) {
	t.Parallel()
	f := newFakeClient()
	s := NewService(f)
	preview, err := s.ChangeGroup(context.Background(), GroupDelete, 7, "", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Irreversible || !preview.RequiresIrreversibleAcknowledgement || preview.CurrentItemCount != 1 {
		t.Fatalf("delete preview=%#v", preview)
	}
	if len(preview.AffectedItems) != 1 || preview.AffectedItems[0].ProductCode != "A005930" || preview.AffectedItems[0].Symbol != "005930" {
		t.Fatalf("delete preview omitted affected items: %#v", preview.AffectedItems)
	}
	_, err = s.ChangeGroup(context.Background(), GroupDelete, 7, "", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err == nil || !strings.Contains(err.Error(), "acknowledge") || f.mutations != 0 {
		t.Fatalf("delete without acknowledgement err=%v mutations=%d", err, f.mutations)
	}
	result, err := s.ChangeGroup(context.Background(), GroupDelete, 7, "", ExecuteOptions{
		Execute: true, Confirm: preview.ConfirmToken, AcknowledgeIrreversible: true,
	})
	if err != nil || !result.Applied || len(f.groups) != 0 {
		t.Fatalf("delete result=%#v groups=%#v err=%v", result, f.groups, err)
	}
}

func TestGroupDeletePreviewBindsAffectedMembership(t *testing.T) {
	t.Parallel()
	f := newFakeClient()
	s := NewService(f)
	preview, err := s.ChangeGroup(context.Background(), GroupDelete, 7, "", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Keep the count unchanged while replacing the affected holding. A delete
	// confirmation must bind the actual membership, not only its length.
	f.items[7] = []domain.WatchlistItem{{ProductCode: "US.AAPL", Symbol: "AAPL"}}
	_, err = s.ChangeGroup(context.Background(), GroupDelete, 7, "", ExecuteOptions{
		Execute: true, Confirm: preview.ConfirmToken, AcknowledgeIrreversible: true,
	})
	if err == nil || !strings.Contains(err.Error(), "confirmation token mismatch") {
		t.Fatalf("stale delete preview err=%v", err)
	}
	if f.mutations != 0 {
		t.Fatalf("stale delete preview mutated state: %d", f.mutations)
	}
}

func TestItemChangePreviewBindsCurrentFolderState(t *testing.T) {
	t.Parallel()
	f := newFakeClient()
	s := NewService(f)
	preview, err := s.ChangeItem(context.Background(), ItemAdd, 7, "AAPL", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ProductCode != "US.AAPL" || preview.ConfirmToken == "" || f.mutations != 0 {
		t.Fatalf("preview=%#v mutations=%d", preview, f.mutations)
	}
	f.items[7] = append(f.items[7], domain.WatchlistItem{ProductCode: "US.MSFT", Symbol: "MSFT"})
	if _, err := s.ChangeItem(context.Background(), ItemAdd, 7, "AAPL", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken}); err == nil {
		t.Fatal("stale item preview was accepted")
	}
	if f.mutations != 0 {
		t.Fatalf("stale preview mutated state: %d", f.mutations)
	}
}

func TestGroupCreateAndNoopRenameExecution(t *testing.T) {
	t.Parallel()
	f := newFakeClient()
	s := NewService(f)

	createPreview, err := s.ChangeGroup(context.Background(), GroupCreate, 0, "Research", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.ChangeGroup(context.Background(), GroupCreate, 0, "Research", ExecuteOptions{
		Execute: true, Confirm: createPreview.ConfirmToken,
	})
	if err != nil || !created.Applied || created.GroupID == 0 || created.GroupName != "Research" || f.mutations != 1 {
		t.Fatalf("created=%#v mutations=%d err=%v", created, f.mutations, err)
	}

	noopPreview, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Long term", ExecuteOptions{})
	if err != nil || !noopPreview.Noop {
		t.Fatalf("noop preview=%#v err=%v", noopPreview, err)
	}
	noop, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Long term", ExecuteOptions{
		Execute: true, Confirm: noopPreview.ConfirmToken,
	})
	if err != nil || !noop.Applied || !noop.Noop || f.mutations != 1 {
		t.Fatalf("noop=%#v mutations=%d err=%v", noop, f.mutations, err)
	}
}

func TestGroupChangeReconcilesTransportErrorAndReportsVerificationFailure(t *testing.T) {
	t.Parallel()
	t.Run("transport error after apply", func(t *testing.T) {
		f := newFakeClient()
		s := NewService(f)
		preview, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		f.mutationErr = errors.New("synthetic connection reset")
		result, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{
			Execute: true, Confirm: preview.ConfirmToken,
		})
		if err != nil || !result.Applied || !result.Reconciled {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})

	t.Run("verification read fails", func(t *testing.T) {
		f := newFakeClient()
		s := NewService(f)
		preview, err := s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		f.listErrAt = 1
		_, err = s.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{
			Execute: true, Confirm: preview.ConfirmToken,
		})
		if err == nil || !strings.Contains(err.Error(), "verify group_rename") || !strings.Contains(err.Error(), "synthetic group read failure") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestItemChangeSuccessNoopAndTransportReconciliation(t *testing.T) {
	t.Parallel()
	t.Run("add success then noop", func(t *testing.T) {
		f := newFakeClient()
		s := NewService(f)
		preview, err := s.ChangeItem(context.Background(), ItemAdd, 7, "AAPL", ExecuteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		result, err := s.ChangeItem(context.Background(), ItemAdd, 7, "AAPL", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
		if err != nil || !result.Applied || f.mutations != 1 {
			t.Fatalf("result=%#v mutations=%d err=%v", result, f.mutations, err)
		}
		noopPreview, err := s.ChangeItem(context.Background(), ItemAdd, 7, "AAPL", ExecuteOptions{})
		if err != nil || !noopPreview.Noop {
			t.Fatalf("noop preview=%#v err=%v", noopPreview, err)
		}
		noop, err := s.ChangeItem(context.Background(), ItemAdd, 7, "AAPL", ExecuteOptions{Execute: true, Confirm: noopPreview.ConfirmToken})
		if err != nil || !noop.Applied || !noop.Noop || f.mutations != 1 {
			t.Fatalf("noop=%#v mutations=%d err=%v", noop, f.mutations, err)
		}
	})

	t.Run("remove reconciles transport error", func(t *testing.T) {
		f := newFakeClient()
		f.items[7] = append(f.items[7], domain.WatchlistItem{ProductCode: "US.AAPL", Symbol: "AAPL"})
		s := NewService(f)
		preview, err := s.ChangeItem(context.Background(), ItemRemove, 7, "AAPL", ExecuteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		f.mutationErr = errors.New("synthetic timeout")
		result, err := s.ChangeItem(context.Background(), ItemRemove, 7, "AAPL", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
		if err != nil || !result.Applied || !result.Reconciled {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
}

func TestWatchlistServiceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		run  func() error
		want string
	}{
		{name: "nil service", run: func() error {
			_, err := (*Service)(nil).ChangeGroup(context.Background(), GroupCreate, 0, "x", ExecuteOptions{})
			return err
		}, want: "not configured"},
		{name: "unsupported group action", run: func() error {
			_, err := NewService(newFakeClient()).ChangeGroup(context.Background(), ItemAdd, 0, "x", ExecuteOptions{})
			return err
		}, want: "unsupported"},
		{name: "empty name", run: func() error {
			_, err := NewService(newFakeClient()).ChangeGroup(context.Background(), GroupCreate, 0, " ", ExecuteOptions{})
			return err
		}, want: "must not be empty"},
		{name: "invalid group id", run: func() error {
			_, err := NewService(newFakeClient()).ChangeGroup(context.Background(), GroupDelete, 0, "", ExecuteOptions{})
			return err
		}, want: "greater than zero"},
		{name: "missing group", run: func() error {
			_, err := NewService(newFakeClient()).ChangeItem(context.Background(), ItemAdd, 99, "AAPL", ExecuteOptions{})
			return err
		}, want: "not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestConfirmationTokenIsSessionBoundAndExpires(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)
	firstClient := newFakeClient()
	secondClient := newFakeClient()
	secondClient.key = []byte("different-session-confirmation-key")
	first := NewService(firstClient)
	second := NewService(secondClient)
	first.now = func() time.Time { return now }
	second.now = func() time.Time { return now }

	firstPreview, err := first.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	secondPreview, err := second.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if firstPreview.ConfirmToken == secondPreview.ConfirmToken {
		t.Fatal("identical state in different sessions produced the same confirmation token")
	}

	now = now.Add(confirmationTTL + time.Second)
	_, err = first.ChangeGroup(context.Background(), GroupRename, 7, "Retirement", ExecuteOptions{Execute: true, Confirm: firstPreview.ConfirmToken})
	if err == nil || !strings.Contains(err.Error(), "confirmation token mismatch") || firstClient.mutations != 0 {
		t.Fatalf("expired confirmation err=%v mutations=%d", err, firstClient.mutations)
	}
}

func TestPreviewDoesNotExposeConfirmationSnapshot(t *testing.T) {
	t.Parallel()
	preview, err := NewService(newFakeClient()).ChangeGroup(context.Background(), GroupCreate, 0, "Research", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"Long term", "A005930", "canonical", "confirmationMaterial", "confirmationKey"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("preview leaked private confirmation snapshot %q: %s", private, encoded)
		}
	}
}
