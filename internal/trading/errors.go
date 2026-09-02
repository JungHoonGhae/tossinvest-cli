package trading

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrExecuteRequired         = errors.New("live trading requires --execute")
	ErrLiveActionsDisabled     = errors.New("live order actions are disabled in config")
	ErrConfirmMismatch         = errors.New("confirmation token mismatch")
	ErrLiveMutationPending     = errors.New("live trading mutation is not implemented yet")
	ErrPlaceUnsupported        = errors.New("live place supports only a narrow subset of orders")
	ErrPlaceNotReconciled      = errors.New("placed order was not found in pending reconciliation")
	ErrCancelStillPending      = errors.New("pending order still present after cancel reconciliation")
	ErrInteractiveAuthRequired = errors.New("broker requires interactive trade authentication")
)

type Branch string

const (
	BranchFundingRequired   Branch = "funding_required"
	BranchFXConsentRequired Branch = "fx_consent_required"
)

type BranchSource string

const (
	BranchSourcePrepareRejection        BranchSource = "prepare_rejection"
	BranchSourcePostPrepareConfirmation BranchSource = "post_prepare_confirmation"
)

type FXConfirmationContext struct {
	NeedExchangeUSD      float64 `json:"need_exchange_usd,omitempty"`
	EstimatedExchangeKRW float64 `json:"estimated_exchange_krw,omitempty"`
	USDExchangeRate      float64 `json:"usd_exchange_rate,omitempty"`
	RateQuoteID          string  `json:"rate_quote_id,omitempty"`
	ValidFrom            string  `json:"valid_from,omitempty"`
	ValidTill            string  `json:"valid_till,omitempty"`
	GettingBackKRW       bool    `json:"getting_back_krw,omitempty"`
	GettingBackKRWKnown  bool    `json:"getting_back_krw_known,omitempty"`
}

type BranchRequiredError struct {
	Branch        Branch
	Source        BranchSource
	StatusCode    int
	BrokerMessage string
	FX            *FXConfirmationContext
}

func (e *BranchRequiredError) Error() string {
	if e == nil {
		return "broker requires operator action"
	}
	if message := strings.TrimSpace(e.BrokerMessage); message != "" {
		return fmt.Sprintf("broker requires %s: %s", e.Branch, message)
	}
	return fmt.Sprintf("broker requires %s", e.Branch)
}

type PrepareRejectedError struct {
	StatusCode    int
	BrokerMessage string
}

func (e *PrepareRejectedError) Error() string {
	if e == nil {
		return "broker rejected order prepare"
	}
	if message := strings.TrimSpace(e.BrokerMessage); message != "" {
		return fmt.Sprintf("broker rejected order prepare (%d): %s", e.StatusCode, message)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("broker rejected order prepare (%d)", e.StatusCode)
	}
	return "broker rejected order prepare"
}

type Action string

const (
	ActionPlace       Action = "place"
	ActionCancel      Action = "cancel"
	ActionAmend       Action = "amend"
	ActionConditional Action = "conditional"
)

type DisabledActionError struct {
	Action Action
}

func (e *DisabledActionError) Error() string {
	return "trading action is disabled in config: " + string(e.Action)
}
