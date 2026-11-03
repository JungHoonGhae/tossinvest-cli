package orderlineage

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/atomicfile"
)

type Entry struct {
	CurrentOrderID string    `json:"current_order_id,omitempty"`
	Kind           string    `json:"kind,omitempty"`
	Symbol         string    `json:"symbol,omitempty"`
	Market         string    `json:"market,omitempty"`
	Quantity       float64   `json:"quantity,omitempty"`
	Price          float64   `json:"price,omitempty"`
	OrderDate      string    `json:"order_date,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type File struct {
	Mappings map[string]Entry `json:"mappings"`
}

type Service struct {
	path string
}

func NewService(path string) *Service {
	return &Service{path: path}
}

func (s *Service) Path() string {
	return s.path
}

func (s *Service) Resolve(orderID string) (string, bool, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return "", false, nil
	}

	state, err := s.load()
	if err != nil {
		return "", false, err
	}

	current := orderID
	seen := map[string]struct{}{}
	for {
		if _, ok := seen[current]; ok {
			break
		}
		seen[current] = struct{}{}

		entry, ok := state.Mappings[current]
		if !ok {
			break
		}
		next := strings.TrimSpace(entry.CurrentOrderID)
		if next == "" || next == current {
			break
		}
		current = next
	}

	if current == orderID {
		return "", false, nil
	}
	return current, true, nil
}

func (s *Service) Lookup(orderID string) (Entry, bool, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return Entry{}, false, nil
	}

	state, err := s.load()
	if err != nil {
		return Entry{}, false, err
	}

	entry, ok := state.Mappings[orderID]
	if !ok {
		return Entry{}, false, nil
	}
	return entry, true, nil
}

func (s *Service) Record(originalOrderID string, entry Entry) error {
	originalOrderID = strings.TrimSpace(originalOrderID)
	entry.CurrentOrderID = strings.TrimSpace(entry.CurrentOrderID)
	entry.Kind = strings.TrimSpace(entry.Kind)
	entry.Symbol = strings.TrimSpace(entry.Symbol)
	entry.Market = strings.TrimSpace(entry.Market)
	entry.OrderDate = strings.TrimSpace(entry.OrderDate)
	if originalOrderID == "" || !entry.meaningful() {
		return nil
	}

	state, err := s.load()
	if err != nil {
		return err
	}
	if state.Mappings == nil {
		state.Mappings = map[string]Entry{}
	}

	now := time.Now().UTC()
	entry.UpdatedAt = now
	state.Mappings[originalOrderID] = entry
	for alias, existing := range state.Mappings {
		if strings.TrimSpace(existing.CurrentOrderID) != originalOrderID {
			continue
		}
		if entry.CurrentOrderID == "" || entry.CurrentOrderID == originalOrderID {
			continue
		}
		existing.CurrentOrderID = entry.CurrentOrderID
		existing.UpdatedAt = now
		if entry.Kind != "" {
			existing.Kind = entry.Kind
		}
		if entry.Symbol != "" {
			existing.Symbol = entry.Symbol
		}
		if entry.Market != "" {
			existing.Market = entry.Market
		}
		if entry.Quantity != 0 {
			existing.Quantity = entry.Quantity
		}
		if entry.Price != 0 {
			existing.Price = entry.Price
		}
		if entry.OrderDate != "" {
			existing.OrderDate = entry.OrderDate
		}
		state.Mappings[alias] = existing
	}

	return s.save(state)
}

func (e Entry) meaningful() bool {
	if e.CurrentOrderID != "" {
		return true
	}
	if e.Kind != "" || e.Symbol != "" || e.Market != "" || e.OrderDate != "" {
		return true
	}
	return e.Quantity != 0 || e.Price != 0
}

func (s *Service) load() (File, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{Mappings: map[string]Entry{}}, nil
		}
		return File{}, err
	}
	if len(data) == 0 {
		return File{Mappings: map[string]Entry{}}, nil
	}

	var state File
	if err := json.Unmarshal(data, &state); err != nil {
		return File{}, err
	}
	if state.Mappings == nil {
		state.Mappings = map[string]Entry{}
	}
	return state, nil
}

func (s *Service) save(state File) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return atomicfile.WriteFile(s.path, data, 0o600)
}
