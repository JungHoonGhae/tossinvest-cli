package client

import "github.com/JungHoonGhae/tossinvest-cli/internal/domain"

type batchRequest struct {
	code   string
	symbol string
}

func batchCodes(requests []batchRequest) []string {
	codes := make([]string, len(requests))
	for i, request := range requests {
		codes[i] = request.code
	}
	return codes
}

// reconcileBatch restores request order after a batch backend response and
// reports every requested code the backend omitted. WTS batch endpoints are
// allowed to reorder and omit rows, so callers must never align by position or
// silently treat a shorter response as complete.
//
// label copies the original caller symbol onto each returned value. Keeping
// the request as ordered (code, symbol) pairs prevents aliases that resolve to
// the same product code from overwriting one another in a reverse map.
func reconcileBatch[T any](requests []batchRequest, returnedByCode map[string]T, label func(T, string) T) ([]T, []string, []domain.BatchSequenceEntry) {
	items := make([]T, 0, len(returnedByCode))
	missing := make([]string, 0)
	sequence := make([]domain.BatchSequenceEntry, 0, len(requests))
	for _, request := range requests {
		item, ok := returnedByCode[request.code]
		if !ok {
			missing = append(missing, request.symbol)
			sequence = append(sequence, domain.BatchSequenceEntry{Symbol: request.symbol, Missing: true})
			continue
		}
		items = append(items, label(item, request.symbol))
		sequence = append(sequence, domain.BatchSequenceEntry{Symbol: request.symbol})
	}
	return items, missing, sequence
}
