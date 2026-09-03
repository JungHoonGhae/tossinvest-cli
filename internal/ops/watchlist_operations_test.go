package ops

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
)

type opsWatchlistClient struct {
	groups    []domain.WatchlistGroup
	mutations int
}

func (c *opsWatchlistClient) ListWatchlistGroups(context.Context) ([]domain.WatchlistGroup, error) {
	return append([]domain.WatchlistGroup(nil), c.groups...), nil
}
func (c *opsWatchlistClient) GetWatchlistGroupItems(context.Context, int64) ([]domain.WatchlistItem, error) {
	return []domain.WatchlistItem{{ProductCode: "A005930", Symbol: "005930"}}, nil
}
func (c *opsWatchlistClient) GetWatchlistGroup(_ context.Context, id int64) (domain.WatchlistGroup, error) {
	for _, group := range c.groups {
		if group.ID == id {
			group.Items = []domain.WatchlistItem{{ProductCode: "A005930", Symbol: "005930"}}
			group.ItemCount = len(group.Items)
			return group, nil
		}
	}
	return domain.WatchlistGroup{}, fmt.Errorf("watchlist folder %d not found", id)
}
func (c *opsWatchlistClient) ConfirmationKey(string) []byte {
	return []byte("ops-watchlist-confirmation-key")
}
func (c *opsWatchlistClient) ResolveProductCode(context.Context, string) (string, error) {
	return "US.AAPL", nil
}
func (c *opsWatchlistClient) CreateWatchlistGroup(context.Context, string) (domain.WatchlistGroup, error) {
	c.mutations++
	return domain.WatchlistGroup{}, nil
}
func (c *opsWatchlistClient) RenameWatchlistGroup(context.Context, int64, string) error {
	c.mutations++
	return nil
}
func (c *opsWatchlistClient) DeleteWatchlistGroup(context.Context, int64) error {
	c.mutations++
	return nil
}
func (c *opsWatchlistClient) AddWatchlistItem(context.Context, int64, string) error {
	c.mutations++
	return nil
}
func (c *opsWatchlistClient) RemoveWatchlistItem(context.Context, int64, string) error {
	c.mutations++
	return nil
}

func TestWatchlistWriteOperationsExposePolicyAndPreviewByDefault(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{
		"watchlist_group_create", "watchlist_group_rename", "watchlist_group_delete",
		"watchlist_item_add", "watchlist_item_remove",
	} {
		op, ok := catalog.Get(id)
		if !ok || !op.Write || op.Mutation == nil || !op.Mutation.RequiresFreshConfirmation {
			t.Errorf("%s: operation=%#v present=%t", id, op, ok)
		}
	}
	deleteOp, _ := catalog.Get("watchlist_group_delete")
	if deleteOp.Mutation.RiskLevel != MutationRiskDestructive || deleteOp.Mutation.Reversibility != MutationIrreversible || !deleteOp.Mutation.RequiresIrreversibleAcknowledgement {
		t.Fatalf("delete policy=%#v", deleteOp.Mutation)
	}
	for _, param := range deleteOp.Params {
		if param.Name == "name" {
			t.Fatalf("delete operation exposes unused name parameter: %#v", deleteOp.Params)
		}
	}

	fake := &opsWatchlistClient{groups: []domain.WatchlistGroup{{ID: 7, Name: "Long term", Type: "USER_MADE", ItemCount: 1}}}
	deps := &Deps{
		WTS:        &hybrid.Client{},
		Watchlists: watchlistservice.NewService(fake),
		Auth:       AuthStatus{WTS: BackendStatus{Connected: true}},
	}
	if _, err := catalog.Call(context.Background(), deps, "watchlist_group_delete", map[string]any{"group_id": 7, "name": "unused"}); err == nil || !strings.Contains(err.Error(), "unknown parameter") {
		t.Fatalf("delete must reject its removed name parameter, got %v", err)
	}
	result, err := catalog.Call(context.Background(), deps, "watchlist_group_rename", map[string]any{"group_id": 7, "name": "Retirement"})
	if err != nil {
		t.Fatal(err)
	}
	preview := result.(watchlistservice.Plan)
	if preview.ConfirmToken == "" || fake.mutations != 0 {
		t.Fatalf("preview=%#v mutations=%d", preview, fake.mutations)
	}

	deleteAny, err := catalog.Call(context.Background(), deps, "watchlist_group_delete", map[string]any{"group_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	deletePreview := deleteAny.(watchlistservice.Plan)
	_, err = catalog.Call(context.Background(), deps, "watchlist_group_delete", map[string]any{
		"group_id": 7, "execute": true, "confirm": deletePreview.ConfirmToken,
	})
	if err == nil || !strings.Contains(err.Error(), "acknowledge") || fake.mutations != 0 {
		t.Fatalf("unguarded delete err=%v mutations=%d", err, fake.mutations)
	}
}

func TestWatchlistWriteReadDependenciesAreMonitored(t *testing.T) {
	t.Parallel()
	probes := map[string]bool{}
	for _, probe := range NewCatalog().Probes() {
		probes[probe.Name] = true
	}
	for _, dependency := range []string{"stock-search", "watchlist", "watchlist-groups", "watchlist-group"} {
		if !probes[dependency] {
			t.Errorf("watchlist write dependency %q is not monitored", dependency)
		}
	}
}

func TestWatchlistGroupProbeRejectsIncompleteFolderShapes(t *testing.T) {
	t.Parallel()
	check := statusAndWatchlistGroup()
	if err := check(http.StatusOK, []byte(`{"result":{"watchlists":[{"id":731,"itemCount":0,"items":[]}]}}`)); err != nil {
		t.Fatalf("valid empty folder rejected: %v", err)
	}
	for name, body := range map[string]string{
		"empty result":  `{"result":{"watchlists":[]}}`,
		"missing id":    `{"result":{"watchlists":[{"items":[]}]}}`,
		"missing items": `{"result":{"watchlists":[{"id":731}]}}`,
		"null items":    `{"result":{"watchlists":[{"id":731,"items":null}]}}`,
		"missing code":  `{"result":{"watchlists":[{"id":731,"items":[{}]}]}}`,
		"missing count": `{"result":{"watchlists":[{"id":731,"items":[]}]}}`,
		"wrong count":   `{"result":{"watchlists":[{"id":731,"itemCount":2,"items":[]}]}}`,
	} {
		if err := check(http.StatusOK, []byte(body)); err == nil {
			t.Errorf("%s schema was accepted", name)
		}
	}

	simple := statusAndWatchlistGroupsSimple()
	if err := simple(http.StatusOK, []byte(`{"result":{"watchlists":[]}}`)); err != nil {
		t.Fatalf("valid empty folder list rejected: %v", err)
	}
	if err := simple(http.StatusOK, []byte(`{"result":{"watchlists":[{"id":731}]}}`)); err == nil {
		t.Fatal("simple folder probe accepted missing itemCount")
	}

	bulk := statusAndWatchlistFolders(false)
	if err := bulk(http.StatusOK, []byte(`{"result":{"watchlists":[]}}`)); err != nil {
		t.Fatalf("valid empty bulk watchlist rejected: %v", err)
	}
	if err := bulk(http.StatusOK, []byte(`{"result":{"watchlists":[{"id":731,"itemCount":0,"items":[]},{"id":732,"itemCount":1}]}}`)); err == nil {
		t.Fatal("bulk probe accepted incomplete second folder")
	}
}
