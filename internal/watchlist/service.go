// Package watchlist owns the safe preview/confirm/verify workflow for Toss
// Securities watchlist folders and their stock membership.
package watchlist

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type Action string

const (
	GroupCreate     Action = "group_create"
	GroupRename     Action = "group_rename"
	GroupDelete     Action = "group_delete"
	ItemAdd         Action = "item_add"
	ItemRemove      Action = "item_remove"
	confirmationTTL        = 5 * time.Minute
)

type Client interface {
	ListWatchlistGroups(context.Context) ([]domain.WatchlistGroup, error)
	GetWatchlistGroup(context.Context, int64) (domain.WatchlistGroup, error)
	ConfirmationKey(string) []byte
	ResolveProductCode(context.Context, string) (string, error)
	CreateWatchlistGroup(context.Context, string) (domain.WatchlistGroup, error)
	RenameWatchlistGroup(context.Context, int64, string) error
	DeleteWatchlistGroup(context.Context, int64) error
	AddWatchlistItem(context.Context, int64, string) error
	RemoveWatchlistItem(context.Context, int64, string) error
}

type ExecuteOptions struct {
	Execute                 bool
	Confirm                 string
	AcknowledgeIrreversible bool
}

// PreviewItem identifies a holding affected by an irreversible folder delete.
// Internal watchlist item keys are deliberately not exposed.
type PreviewItem struct {
	ProductCode string `json:"product_code"`
	Symbol      string `json:"symbol,omitempty"`
	Name        string `json:"name,omitempty"`
}

// Plan is both a dry-run result and the verified execution result. The token
// binds the requested action to a freshly-read snapshot of the affected
// folder, so an intervening edit invalidates an old preview.
type Plan struct {
	Kind                                string        `json:"kind"`
	Action                              Action        `json:"action"`
	GroupID                             int64         `json:"group_id,omitempty"`
	GroupName                           string        `json:"group_name,omitempty"`
	NewName                             string        `json:"new_name,omitempty"`
	ProductCode                         string        `json:"product_code,omitempty"`
	CurrentItemCount                    int           `json:"current_item_count,omitempty"`
	AffectedItems                       []PreviewItem `json:"affected_items,omitempty"`
	Irreversible                        bool          `json:"irreversible"`
	RequiresIrreversibleAcknowledgement bool          `json:"requires_irreversible_acknowledgement"`
	Noop                                bool          `json:"noop"`
	Applied                             bool          `json:"applied"`
	Reconciled                          bool          `json:"reconciled,omitempty"`
	ConfirmToken                        string        `json:"confirm_token"`
	beforeGroupIDs                      []int64
	confirmationMaterial                string
	confirmationKey                     []byte
}

type Service struct {
	client Client
	now    func() time.Time
}

func NewService(client Client) *Service { return &Service{client: client, now: time.Now} }

func (s *Service) ChangeGroup(ctx context.Context, action Action, groupID int64, name string, opts ExecuteOptions) (Plan, error) {
	if s == nil || s.client == nil {
		return Plan{}, fmt.Errorf("watchlist manager is not configured")
	}
	name = strings.TrimSpace(name)
	if action != GroupCreate && action != GroupRename && action != GroupDelete {
		return Plan{}, fmt.Errorf("unsupported watchlist group action %q", action)
	}
	if (action == GroupCreate || action == GroupRename) && name == "" {
		return Plan{}, fmt.Errorf("watchlist folder name must not be empty")
	}
	if action != GroupCreate && groupID <= 0 {
		return Plan{}, fmt.Errorf("watchlist folder id must be greater than zero")
	}

	plan, err := s.buildGroupPlan(ctx, action, groupID, name)
	if err != nil {
		return Plan{}, err
	}
	if !opts.Execute {
		return plan, nil
	}
	if !confirmation.VerifyTimeBound(opts.Confirm, plan.confirmationKey, plan.confirmationMaterial, s.now()) {
		return Plan{}, fmt.Errorf("confirmation token mismatch; preview again and pass its confirm_token")
	}
	if plan.RequiresIrreversibleAcknowledgement && !opts.AcknowledgeIrreversible {
		return Plan{}, fmt.Errorf("this deletion is irreversible; pass acknowledge_irreversible=true (CLI: --acknowledge-irreversible) after reviewing the preview")
	}
	if plan.Noop {
		plan.Applied = true
		return plan, nil
	}

	var mutateErr error
	switch action {
	case GroupCreate:
		var created domain.WatchlistGroup
		created, mutateErr = s.client.CreateWatchlistGroup(ctx, name)
		if mutateErr == nil {
			plan.GroupID = created.ID
			plan.GroupName = created.Name
		}
	case GroupRename:
		mutateErr = s.client.RenameWatchlistGroup(ctx, groupID, name)
	case GroupDelete:
		mutateErr = s.client.DeleteWatchlistGroup(ctx, groupID)
	}

	verifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	applied, reconcileID, verifyErr := s.groupDesiredState(verifyCtx, plan)
	if applied {
		if plan.GroupID == 0 {
			plan.GroupID = reconcileID
		}
		plan.Applied = true
		plan.Reconciled = mutateErr != nil
		return plan, nil
	}
	if mutateErr != nil {
		return Plan{}, fmt.Errorf("%s watchlist folder: %w; post-write verification did not observe the requested state", action, mutateErr)
	}
	if verifyErr != nil {
		return Plan{}, fmt.Errorf("verify %s watchlist folder: %w", action, verifyErr)
	}
	return Plan{}, fmt.Errorf("verify %s watchlist folder: requested state was not observed", action)
}

func (s *Service) ChangeItem(ctx context.Context, action Action, groupID int64, symbol string, opts ExecuteOptions) (Plan, error) {
	if s == nil || s.client == nil {
		return Plan{}, fmt.Errorf("watchlist manager is not configured")
	}
	if action != ItemAdd && action != ItemRemove {
		return Plan{}, fmt.Errorf("unsupported watchlist item action %q", action)
	}
	if groupID <= 0 {
		return Plan{}, fmt.Errorf("watchlist folder id must be greater than zero")
	}
	productCode, err := s.client.ResolveProductCode(ctx, strings.TrimSpace(symbol))
	if err != nil {
		return Plan{}, err
	}
	plan, err := s.buildItemPlan(ctx, action, groupID, productCode)
	if err != nil {
		return Plan{}, err
	}
	if !opts.Execute {
		return plan, nil
	}
	if !confirmation.VerifyTimeBound(opts.Confirm, plan.confirmationKey, plan.confirmationMaterial, s.now()) {
		return Plan{}, fmt.Errorf("confirmation token mismatch; preview again and pass its confirm_token")
	}
	if plan.Noop {
		plan.Applied = true
		return plan, nil
	}

	var mutateErr error
	if action == ItemAdd {
		mutateErr = s.client.AddWatchlistItem(ctx, groupID, productCode)
	} else {
		mutateErr = s.client.RemoveWatchlistItem(ctx, groupID, productCode)
	}
	verifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	applied, verifyErr := s.itemDesiredState(verifyCtx, plan)
	if applied {
		plan.Applied = true
		plan.Reconciled = mutateErr != nil
		return plan, nil
	}
	if mutateErr != nil {
		return Plan{}, fmt.Errorf("%s watchlist item: %w; post-write verification did not observe the requested state", action, mutateErr)
	}
	if verifyErr != nil {
		return Plan{}, fmt.Errorf("verify %s watchlist item: %w", action, verifyErr)
	}
	return Plan{}, fmt.Errorf("verify %s watchlist item: requested state was not observed", action)
}

func (s *Service) buildGroupPlan(ctx context.Context, action Action, groupID int64, name string) (Plan, error) {
	plan := Plan{Kind: "securities_watchlist_change", Action: action, NewName: name}
	var groups []domain.WatchlistGroup
	var affectedItems []domain.WatchlistItem
	if action == GroupCreate {
		var err error
		groups, err = s.client.ListWatchlistGroups(ctx)
		if err != nil {
			return Plan{}, err
		}
		sortGroups(groups)
		for _, group := range groups {
			plan.beforeGroupIDs = append(plan.beforeGroupIDs, group.ID)
		}
	} else {
		group, err := s.client.GetWatchlistGroup(ctx, groupID)
		if err != nil {
			return Plan{}, err
		}
		groups = []domain.WatchlistGroup{group}
		plan.GroupID, plan.GroupName, plan.CurrentItemCount = group.ID, group.Name, group.ItemCount
		affectedItems = group.Items
		if action == GroupRename {
			plan.Noop = group.Name == name
		}
		if action == GroupDelete {
			plan.CurrentItemCount = len(group.Items)
			plan.AffectedItems = previewItems(group.Items)
			plan.Irreversible = true
			plan.RequiresIrreversibleAcknowledgement = true
		}
	}
	return s.finalizePlan(plan, groups, affectedItems)
}

func (s *Service) buildItemPlan(ctx context.Context, action Action, groupID int64, productCode string) (Plan, error) {
	group, err := s.client.GetWatchlistGroup(ctx, groupID)
	if err != nil {
		return Plan{}, err
	}
	items := group.Items
	present := containsProduct(items, productCode)
	plan := Plan{
		Kind: "securities_watchlist_change", Action: action, GroupID: groupID,
		GroupName: group.Name, ProductCode: productCode, CurrentItemCount: len(items),
		Noop: (action == ItemAdd && present) || (action == ItemRemove && !present),
	}
	return s.finalizePlan(plan, nil, items)
}

func (s *Service) finalizePlan(plan Plan, groups []domain.WatchlistGroup, items []domain.WatchlistItem) (Plan, error) {
	type groupSnapshot struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		ItemCount int    `json:"item_count"`
	}
	groupState := make([]groupSnapshot, 0, len(groups))
	for _, group := range groups {
		groupState = append(groupState, groupSnapshot{group.ID, group.Name, group.Type, group.ItemCount})
	}
	itemCodes := productCodes(items)
	encoded, err := json.Marshal(struct {
		Kind                                string   `json:"kind"`
		Action                              Action   `json:"action"`
		GroupID                             int64    `json:"group_id,omitempty"`
		GroupName                           string   `json:"group_name,omitempty"`
		NewName                             string   `json:"new_name,omitempty"`
		ProductCode                         string   `json:"product_code,omitempty"`
		Irreversible                        bool     `json:"irreversible"`
		RequiresIrreversibleAcknowledgement bool     `json:"requires_irreversible_acknowledgement"`
		Noop                                bool     `json:"noop"`
		Groups                              any      `json:"groups,omitempty"`
		ItemCodes                           []string `json:"item_codes,omitempty"`
	}{plan.Kind, plan.Action, plan.GroupID, plan.GroupName, plan.NewName, plan.ProductCode,
		plan.Irreversible, plan.RequiresIrreversibleAcknowledgement, plan.Noop, groupState, itemCodes})
	if err != nil {
		return Plan{}, err
	}
	plan.confirmationMaterial = string(encoded)
	plan.confirmationKey = s.client.ConfirmationKey("securities-watchlist")
	plan.ConfirmToken, err = confirmation.IssueTimeBound(plan.confirmationKey, plan.confirmationMaterial, s.now(), confirmationTTL)
	if err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func (s *Service) groupDesiredState(ctx context.Context, plan Plan) (bool, int64, error) {
	groups, err := s.client.ListWatchlistGroups(ctx)
	if err != nil {
		return false, 0, err
	}
	switch plan.Action {
	case GroupCreate:
		if plan.GroupID > 0 {
			group, ok := findGroup(groups, plan.GroupID)
			return ok && group.Name == plan.NewName, plan.GroupID, nil
		}
		before := make(map[int64]bool, len(plan.beforeGroupIDs))
		for _, id := range plan.beforeGroupIDs {
			before[id] = true
		}
		var matched int64
		for _, group := range groups {
			if !before[group.ID] && group.Name == plan.NewName {
				if matched != 0 {
					return false, 0, nil
				}
				matched = group.ID
			}
		}
		return matched != 0, matched, nil
	case GroupRename:
		group, ok := findGroup(groups, plan.GroupID)
		return ok && group.Name == plan.NewName, plan.GroupID, nil
	case GroupDelete:
		_, ok := findGroup(groups, plan.GroupID)
		return !ok, plan.GroupID, nil
	default:
		return false, 0, nil
	}
}

func (s *Service) itemDesiredState(ctx context.Context, plan Plan) (bool, error) {
	group, err := s.client.GetWatchlistGroup(ctx, plan.GroupID)
	if err != nil {
		return false, err
	}
	present := containsProduct(group.Items, plan.ProductCode)
	return (plan.Action == ItemAdd && present) || (plan.Action == ItemRemove && !present), nil
}

func sortGroups(groups []domain.WatchlistGroup) {
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
}

func findGroup(groups []domain.WatchlistGroup, id int64) (domain.WatchlistGroup, bool) {
	for _, group := range groups {
		if group.ID == id {
			return group, true
		}
	}
	return domain.WatchlistGroup{}, false
}

func containsProduct(items []domain.WatchlistItem, code string) bool {
	for _, item := range items {
		if item.ProductCode == code || (item.ProductCode == "" && item.Symbol == code) {
			return true
		}
	}
	return false
}

func productCodes(items []domain.WatchlistItem) []string {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		code := item.ProductCode
		if code == "" {
			code = item.Symbol
		}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func previewItems(items []domain.WatchlistItem) []PreviewItem {
	out := make([]PreviewItem, 0, len(items))
	for _, item := range items {
		code := item.ProductCode
		if code == "" {
			code = item.Symbol
		}
		out = append(out, PreviewItem{ProductCode: code, Symbol: item.Symbol, Name: item.Name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProductCode != out[j].ProductCode {
			return out[i].ProductCode < out[j].ProductCode
		}
		return out[i].Name < out[j].Name
	})
	return out
}
