package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type Reader interface {
	ListAccounts(context.Context) ([]domain.Account, string, error)
	ConfirmationKey(string) []byte
	ListPositions(context.Context) ([]domain.Position, error)
	ListTransactions(context.Context, string, time.Time, time.Time, string, int, int) (domain.TransactionPage, error)
}

type Service struct {
	Store  Store
	Reader Reader
	now    func() time.Time
}

func New(path string, reader Reader) *Service {
	return &Service{Store: Store{Path: path}, Reader: reader, now: time.Now}
}

type SyncOptions struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Market    string `json:"market"`
	PageLimit int    `json:"page_limit"`
}

type SyncResult struct {
	DryRun           bool        `json:"dry_run"`
	Database         string      `json:"database"`
	Intent           SyncOptions `json:"intent"`
	PreviousSnapshot int64       `json:"previous_snapshot"`
	ConfirmToken     string      `json:"confirm_token,omitempty"`
	Snapshot         *Snapshot   `json:"snapshot,omitempty"`
	Note             string      `json:"note"`
}

func validateDates(from, to string, required bool) error {
	for _, v := range []string{from, to} {
		if v == "" && !required {
			continue
		}
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return fmt.Errorf("dates must use YYYY-MM-DD")
		}
	}
	if from != "" && to != "" && from > to {
		return fmt.Errorf("from must be on or before to")
	}
	return nil
}

func (o SyncOptions) normalize(now time.Time) (SyncOptions, error) {
	if o.To == "" {
		o.To = now.In(tossclient.KoreaLocation).Format("2006-01-02")
	}
	to, err := time.ParseInLocation("2006-01-02", o.To, tossclient.KoreaLocation)
	if err != nil {
		return o, fmt.Errorf("to must use YYYY-MM-DD")
	}
	if o.From == "" {
		o.From = to.AddDate(0, 0, -30).Format("2006-01-02")
	}
	if err := validateDates(o.From, o.To, true); err != nil {
		return o, err
	}
	from, _ := time.ParseInLocation("2006-01-02", o.From, tossclient.KoreaLocation)
	if int(to.Sub(from).Hours()/24)+1 > tossclient.TransactionsMaxRangeDays {
		return o, fmt.Errorf("sync date range exceeds %d days; collect separate ranges", tossclient.TransactionsMaxRangeDays)
	}
	if o.Market == "" {
		o.Market = "all"
	}
	if o.Market != "all" && o.Market != "kr" && o.Market != "us" {
		return o, fmt.Errorf("market must be all, kr, or us")
	}
	if o.PageLimit == 0 {
		o.PageLimit = 20
	}
	if o.PageLimit < 1 || o.PageLimit > 200 {
		return o, fmt.Errorf("page-limit must be 1..200")
	}
	return o, nil
}

// Sync previews a local append, binding the account set, primary account,
// exact collection intent, database path, and last committed snapshot. Only
// execute with its fresh token fetches market data and writes the local store.
func (s *Service) Sync(ctx context.Context, o SyncOptions, execute bool, confirm string) (SyncResult, error) {
	var result SyncResult
	var err error
	o, err = o.normalize(s.now())
	if err != nil {
		return result, err
	}
	if s.Reader == nil {
		return result, fmt.Errorf("history sync needs a Toss web session")
	}
	accounts, primary, err := s.Reader.ListAccounts(ctx)
	if err != nil {
		return result, err
	}
	if len(accounts) == 0 || primary == "" {
		return result, fmt.Errorf("cannot bind history to an empty account scope")
	}
	ids := make([]string, 0, len(accounts))
	for _, a := range accounts {
		if a.ID == "" {
			return result, fmt.Errorf("account scope contains an empty ID")
		}
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)
	identity, _ := json.Marshal(struct {
		Accounts []string
		Primary  string
	}{ids, primary})
	owner := digest(identity)
	revision, err := s.Store.revision(ctx, owner)
	if err != nil {
		return result, err
	}
	path, err := filepath.Abs(s.Store.Path)
	if err != nil {
		return result, err
	}
	canonical, _ := json.Marshal(struct {
		Path, Owner string
		Revision    int64
		Intent      SyncOptions
	}{path, owner, revision, o})
	key := s.Reader.ConfirmationKey("history-sync")
	result = SyncResult{DryRun: !execute, Database: path, Intent: o, PreviousSnapshot: revision, Note: "Append one WTS observation: all session holdings plus transactions for the selected markets and dates. No account changes. Local reads use explicit snapshot IDs; this is not a live trading data source."}
	if !execute {
		result.ConfirmToken, err = confirmation.IssueTimeBound(key, string(canonical), s.now(), 5*time.Minute)
		return result, err
	}
	if !confirmation.VerifyTimeBound(confirm, key, string(canonical), s.now()) {
		return result, fmt.Errorf("history sync confirmation is missing, expired, or no longer matches; preview again")
	}
	positions, err := s.Reader.ListPositions(ctx)
	if err != nil {
		return result, err
	}
	markets := []string{o.Market}
	if o.Market == "all" {
		markets = []string{"kr", "us"}
	}
	data := Data{Snapshot: Snapshot{CollectedAt: s.now().UTC().Format(time.RFC3339Nano), Source: "wts", From: o.From, To: o.To, Markets: markets, Complete: true, Warnings: []string{}}, Positions: positions, Transactions: []Transaction{}}
	for _, market := range markets {
		items, reason := s.collectTransactions(ctx, market, o)
		if err := ctx.Err(); err != nil {
			return result, err
		}
		data.Transactions = append(data.Transactions, items...)
		if reason != "" {
			data.Snapshot.Complete = false
			data.Snapshot.Warnings = append(data.Snapshot.Warnings, market+": "+reason)
		}
	}
	meta, err := s.Store.save(ctx, owner, revision, data)
	if err != nil {
		return result, err
	}
	result.Snapshot = &meta
	return result, nil
}

func digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func transaction(t domain.Transaction) Transaction {
	date := t.Date
	if date == "" && len(t.DateTime) >= 10 {
		date = t.DateTime[:10]
	}
	return Transaction{Key: t.SortKey, Date: date, DateTime: t.DateTime, Market: t.Market, Currency: t.Currency, Category: t.Category, Type: t.Type, StockCode: t.StockCode, StockName: t.StockName, Summary: t.Summary, Quantity: t.Quantity, Amount: t.Amount, AdjustedAmount: t.AdjustedAmount, Commission: t.CommissionAmount, Tax: t.TaxAmount}
}

func (s *Service) collectTransactions(ctx context.Context, market string, o SyncOptions) ([]Transaction, string) {
	from, _ := time.ParseInLocation("2006-01-02", o.From, tossclient.KoreaLocation)
	to, _ := time.ParseInLocation("2006-01-02", o.To, tossclient.KoreaLocation)
	currentTo := to
	seen := map[string]bool{}
	out := []Transaction{}
	for pageNo := 0; pageNo < o.PageLimit; pageNo++ {
		page, err := s.Reader.ListTransactions(ctx, market, from, currentTo, "all", tossclient.TransactionsDefaultPageSize, 0)
		if err != nil {
			return out, "transaction request failed; collection is incomplete"
		}
		earliest := ""
		occurrences := map[string]int{}
		for _, raw := range page.Items {
			t := transaction(raw)
			if t.Market == "" {
				t.Market = market
			}
			if _, err := time.Parse("2006-01-02", t.Date); err == nil && (earliest == "" || t.Date < earliest) {
				earliest = t.Date
			}
			if t.Key == "" {
				b, _ := json.Marshal(t)
				base := digest(b)
				occurrences[base]++
				t.Key = fmt.Sprintf("content:%s:%d", base, occurrences[base])
			}
			if !seen[t.Key] {
				seen[t.Key] = true
				out = append(out, t)
			}
		}
		if page.LastPage {
			return out, ""
		}
		if earliest == "" {
			return out, "server returned a non-terminal page without a usable date"
		}
		next, _ := time.ParseInLocation("2006-01-02", earliest, tossclient.KoreaLocation)
		if !next.Before(currentTo) {
			return out, "date cursor stopped advancing; same-day transactions may be missing"
		}
		if next.Before(from) {
			return out, "server returned dates outside the requested range"
		}
		currentTo = next
	}
	return out, "page limit reached; rerun sync with a larger page-limit or a smaller date range"
}
