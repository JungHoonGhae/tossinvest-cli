package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS snapshots (
 id INTEGER PRIMARY KEY AUTOINCREMENT, collected_at TEXT NOT NULL,
 source TEXT NOT NULL, range_from TEXT NOT NULL, range_to TEXT NOT NULL,
 markets TEXT NOT NULL, complete INTEGER NOT NULL, warnings TEXT NOT NULL,
 position_count INTEGER NOT NULL, transaction_count INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS positions (
 snapshot_id INTEGER NOT NULL REFERENCES snapshots(id), ordinal INTEGER NOT NULL,
 product_code TEXT NOT NULL, symbol TEXT NOT NULL, name TEXT NOT NULL,
 market_type TEXT NOT NULL, market_code TEXT NOT NULL, quantity REAL NOT NULL,
 average_price REAL NOT NULL, current_price REAL NOT NULL, market_value REAL NOT NULL,
 unrealized_pnl REAL NOT NULL, profit_rate REAL NOT NULL, daily_profit_loss REAL NOT NULL,
 daily_profit_rate REAL NOT NULL, average_price_usd REAL NOT NULL, current_price_usd REAL NOT NULL,
 market_value_usd REAL NOT NULL, unrealized_pnl_usd REAL NOT NULL, profit_rate_usd REAL NOT NULL,
 daily_profit_loss_usd REAL NOT NULL, daily_profit_rate_usd REAL NOT NULL,
 PRIMARY KEY(snapshot_id, ordinal));
CREATE TABLE IF NOT EXISTS transactions (
 snapshot_id INTEGER NOT NULL REFERENCES snapshots(id), ordinal INTEGER NOT NULL,
 tx_key TEXT NOT NULL, date TEXT NOT NULL, datetime TEXT NOT NULL, market TEXT NOT NULL,
 currency TEXT NOT NULL, category TEXT NOT NULL, type TEXT NOT NULL, stock_code TEXT NOT NULL,
 stock_name TEXT NOT NULL, summary TEXT NOT NULL, quantity REAL NOT NULL, amount REAL NOT NULL,
 adjusted_amount REAL NOT NULL, commission REAL NOT NULL, tax REAL NOT NULL,
 PRIMARY KEY(snapshot_id, ordinal));
CREATE INDEX IF NOT EXISTS transactions_date ON transactions(snapshot_id, date, market);
PRAGMA user_version=1;`

var errEmptyHistory = errors.New("empty history database")

type Store struct{ Path string }

func (s Store) open(ctx context.Context, write bool) (*sql.DB, error) {
	path, err := filepath.Abs(s.Path)
	if err != nil {
		return nil, err
	}
	if s.Path == "" {
		return nil, fmt.Errorf("history database path is not configured")
	}
	if write {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		if err := file.Chmod(0o600); err != nil {
			file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	} else if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// SQLite file URIs use forward slashes and /C:/ for absolute Windows
	// drive paths. URL.Path also escapes literal ?, # and % in filenames.
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath}
	params := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)", "foreign_keys(1)"}}
	if write {
		params.Set("mode", "rw")
		params.Set("_txlock", "immediate")
	}
	uri.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version > 1 {
		db.Close()
		return nil, fmt.Errorf("history schema %d is newer than this tossctl supports", version)
	}
	if version == 0 {
		var tables int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
			db.Close()
			return nil, err
		}
		if tables != 0 {
			db.Close()
			return nil, fmt.Errorf("file is not a recognized tossctl history database")
		}
		if !write {
			db.Close()
			return nil, errEmptyHistory
		}
	}
	if write && version == 0 {
		if _, err := db.ExecContext(ctx, "BEGIN IMMEDIATE;"+schema+"COMMIT;"); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

func (s Store) revision(ctx context.Context, owner string) (int64, error) {
	db, err := s.open(ctx, false)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errEmptyHistory) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return revision(ctx, db, owner)
}

type rowReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func revision(ctx context.Context, db rowReader, owner string) (int64, error) {
	var stored string
	err := db.QueryRowContext(ctx, "SELECT value FROM metadata WHERE key='owner'").Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if stored != "" && stored != owner {
		return 0, fmt.Errorf("history belongs to a different WTS account scope; use a separate --config-dir")
	}
	var id int64
	err = db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM snapshots").Scan(&id)
	return id, err
}

// save commits a whole observation atomically and rejects a stale preview even
// when another tossctl process synced during the network fetch.
func (s Store) save(ctx context.Context, owner string, expected int64, data Data) (Snapshot, error) {
	// JSON validation rejects NaN/Inf before opening or creating the database.
	if _, err := json.Marshal(data); err != nil {
		return Snapshot{}, err
	}
	db, err := s.open(ctx, true)
	if err != nil {
		return Snapshot{}, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	id, err := revision(ctx, tx, owner)
	if err != nil {
		return Snapshot{}, err
	}
	if id != expected {
		return Snapshot{}, fmt.Errorf("history changed since preview; preview sync again")
	}
	if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO metadata(key,value) VALUES('owner',?)", owner); err != nil {
		return Snapshot{}, err
	}
	meta := data.Snapshot
	meta.PositionCount, meta.TransactionCount = len(data.Positions), len(data.Transactions)
	markets, _ := json.Marshal(meta.Markets)
	warnings, _ := json.Marshal(meta.Warnings)
	result, err := tx.ExecContext(ctx, `INSERT INTO snapshots(collected_at,source,range_from,range_to,markets,complete,warnings,position_count,transaction_count) VALUES(?,?,?,?,?,?,?,?,?)`, meta.CollectedAt, meta.Source, meta.From, meta.To, string(markets), meta.Complete, string(warnings), meta.PositionCount, meta.TransactionCount)
	if err != nil {
		return Snapshot{}, err
	}
	meta.ID, err = result.LastInsertId()
	if err != nil {
		return Snapshot{}, err
	}
	for i, p := range data.Positions {
		_, err = tx.ExecContext(ctx, `INSERT INTO positions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, meta.ID, i, p.ProductCode, p.Symbol, p.Name, p.MarketType, p.MarketCode, p.Quantity, p.AveragePrice, p.CurrentPrice, p.MarketValue, p.UnrealizedPnL, p.ProfitRate, p.DailyProfitLoss, p.DailyProfitRate, p.AveragePriceUSD, p.CurrentPriceUSD, p.MarketValueUSD, p.UnrealizedPnLUSD, p.ProfitRateUSD, p.DailyProfitLossUSD, p.DailyProfitRateUSD)
		if err != nil {
			return Snapshot{}, err
		}
	}
	for i, t := range data.Transactions {
		_, err = tx.ExecContext(ctx, `INSERT INTO transactions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, meta.ID, i, t.Key, t.Date, t.DateTime, t.Market, t.Currency, t.Category, t.Type, t.StockCode, t.StockName, t.Summary, t.Quantity, t.Amount, t.AdjustedAmount, t.Commission, t.Tax)
		if err != nil {
			return Snapshot{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	return meta, nil
}

const snapshotColumns = "id,collected_at,source,range_from,range_to,markets,complete,warnings,position_count,transaction_count"

type scanner interface{ Scan(...any) error }

func scanSnapshot(row scanner) (Snapshot, error) {
	var s Snapshot
	var markets, warnings string
	err := row.Scan(&s.ID, &s.CollectedAt, &s.Source, &s.From, &s.To, &markets, &s.Complete, &warnings, &s.PositionCount, &s.TransactionCount)
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal([]byte(markets), &s.Markets); err != nil {
		return s, err
	}
	err = json.Unmarshal([]byte(warnings), &s.Warnings)
	return s, err
}

func (s Store) List(ctx context.Context, limit int) ([]Snapshot, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	out := []Snapshot{}
	db, err := s.open(ctx, false)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errEmptyHistory) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT "+snapshotColumns+" FROM snapshots ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		meta, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, meta)
	}
	return out, rows.Err()
}

func loadSnapshot(ctx context.Context, db *sql.DB, id int64) (Snapshot, error) {
	query := "SELECT " + snapshotColumns + " FROM snapshots WHERE id=?"
	args := []any{id}
	if id == 0 {
		query = "SELECT " + snapshotColumns + " FROM snapshots ORDER BY id DESC LIMIT 1"
		args = nil
	}
	meta, err := scanSnapshot(db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return meta, fmt.Errorf("history snapshot %d not found; run history list or preview history sync", id)
	}
	return meta, err
}

func (s Store) Positions(ctx context.Context, id int64) (Holdings, error) {
	out := Holdings{Positions: []domain.Position{}}
	db, err := s.open(ctx, false)
	if err != nil {
		return out, historyReadError(err)
	}
	defer db.Close()
	out.Snapshot, err = loadSnapshot(ctx, db, id)
	if err != nil {
		return out, err
	}
	rows, err := db.QueryContext(ctx, "SELECT * FROM positions WHERE snapshot_id=? ORDER BY ordinal", out.Snapshot.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.Position
		var sid int64
		var ordinal int
		err = rows.Scan(&sid, &ordinal, &p.ProductCode, &p.Symbol, &p.Name, &p.MarketType, &p.MarketCode, &p.Quantity, &p.AveragePrice, &p.CurrentPrice, &p.MarketValue, &p.UnrealizedPnL, &p.ProfitRate, &p.DailyProfitLoss, &p.DailyProfitRate, &p.AveragePriceUSD, &p.CurrentPriceUSD, &p.MarketValueUSD, &p.UnrealizedPnLUSD, &p.ProfitRateUSD, &p.DailyProfitLossUSD, &p.DailyProfitRateUSD)
		if err != nil {
			return out, err
		}
		out.Positions = append(out.Positions, p)
	}
	return out, rows.Err()
}

func (s Store) Transactions(ctx context.Context, q Query) (Transactions, error) {
	out := Transactions{Items: []Transaction{}}
	if q.Limit < 1 || q.Limit > 1000 || q.Offset < 0 {
		return out, fmt.Errorf("limit must be 1..1000 and offset must be non-negative")
	}
	if q.Market != "" && q.Market != "kr" && q.Market != "us" {
		return out, fmt.Errorf("market must be kr or us")
	}
	if err := validateDates(q.From, q.To, false); err != nil {
		return out, err
	}
	db, err := s.open(ctx, false)
	if err != nil {
		return out, historyReadError(err)
	}
	defer db.Close()
	out.Snapshot, err = loadSnapshot(ctx, db, q.SnapshotID)
	if err != nil {
		return out, err
	}
	where := "snapshot_id=?"
	args := []any{out.Snapshot.ID}
	if q.From != "" {
		where += " AND date>=?"
		args = append(args, q.From)
	}
	if q.To != "" {
		where += " AND date<=?"
		args = append(args, q.To)
	}
	if q.Market != "" {
		where += " AND market=?"
		args = append(args, q.Market)
	}
	if q.Search != "" {
		where += " AND (instr(lower(stock_code),lower(?))>0 OR instr(lower(stock_name),lower(?))>0 OR instr(lower(summary),lower(?))>0 OR instr(lower(category),lower(?))>0)"
		args = append(args, q.Search, q.Search, q.Search, q.Search)
	}
	args = append(args, q.Limit+1, q.Offset)
	rows, err := db.QueryContext(ctx, "SELECT * FROM transactions WHERE "+where+" ORDER BY date DESC,datetime DESC,ordinal LIMIT ? OFFSET ?", args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Transaction
		var id int64
		var ordinal int
		err = rows.Scan(&id, &ordinal, &t.Key, &t.Date, &t.DateTime, &t.Market, &t.Currency, &t.Category, &t.Type, &t.StockCode, &t.StockName, &t.Summary, &t.Quantity, &t.Amount, &t.AdjustedAmount, &t.Commission, &t.Tax)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, t)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(out.Items) > q.Limit {
		out.HasMore = true
		out.Items = out.Items[:q.Limit]
		out.NextOffset = q.Offset + q.Limit
	}
	return out, nil
}

func historyReadError(err error) error {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errEmptyHistory) {
		return fmt.Errorf("no local history; run `tossctl history sync` to preview a collection")
	}
	return err
}

func (s Store) Compare(ctx context.Context, before, after int64) (Comparison, error) {
	var out Comparison
	if before < 1 || after < 1 || before >= after {
		return out, fmt.Errorf("compare requires two snapshot IDs with before < after")
	}
	a, err := s.Positions(ctx, before)
	if err != nil {
		return out, err
	}
	b, err := s.Positions(ctx, after)
	if err != nil {
		return out, err
	}
	out.Before, out.After, out.Changes = a.Snapshot, b.Snapshot, []Change{}
	out.Note = "Position value changes include trades and price/FX movements; they are not investment returns. Both snapshots are historical observations, not live balances."
	key := func(p domain.Position) string {
		code := p.ProductCode
		if code == "" {
			code = p.Symbol
		}
		return p.MarketType + "/" + p.MarketCode + "/" + code
	}
	old, current := map[string]domain.Position{}, map[string]domain.Position{}
	for _, p := range a.Positions {
		k := key(p)
		if _, exists := old[k]; exists {
			return out, fmt.Errorf("ambiguous duplicate holding %s in snapshot %d", p.Symbol, before)
		}
		old[k] = p
	}
	for _, p := range b.Positions {
		k := key(p)
		if _, exists := current[k]; exists {
			return out, fmt.Errorf("ambiguous duplicate holding %s in snapshot %d", p.Symbol, after)
		}
		current[k] = p
	}
	keys := map[string]bool{}
	for k := range old {
		keys[k] = true
	}
	for k := range current {
		keys[k] = true
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	for _, k := range ordered {
		p, was := old[k]
		n, is := current[k]
		label := n
		status := "changed"
		if !was {
			status = "added"
		}
		if !is {
			status = "removed"
			label = p
		}
		if was && is && p == n {
			continue
		}
		out.Changes = append(out.Changes, Change{Symbol: label.Symbol, Name: label.Name, ProductCode: label.ProductCode, Status: status, QuantityBefore: p.Quantity, QuantityAfter: n.Quantity, QuantityDelta: n.Quantity - p.Quantity, ValueDeltaKRW: n.MarketValue - p.MarketValue, ValueDeltaUSD: n.MarketValueUSD - p.MarketValueUSD, PnLDeltaKRW: n.UnrealizedPnL - p.UnrealizedPnL})
	}
	return out, nil
}
