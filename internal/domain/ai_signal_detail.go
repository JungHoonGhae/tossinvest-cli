package domain

import "time"

// AISignalIssue identifies the asset and observed facts behind one detailed
// Toss AI signal. Description is an ordered list because the upstream payload
// presents it as separately rendered narrative lines.
type AISignalIssue struct {
	AssetCode      string   `json:"asset_code"`
	AssetName      string   `json:"asset_name"`
	AssetType      string   `json:"asset_type"`
	Description    []string `json:"description"`
	InvestmentType string   `json:"investment_type,omitempty"`
	LogoImageURL   string   `json:"logo_image_url,omitempty"`
	OriginCodes    []string `json:"origin_codes"`
	ProfitLossRate float64  `json:"profit_loss_rate,omitempty"`
}

type AISignalRelationship struct {
	SubjectName string `json:"subject_name,omitempty"`
	Relation    string `json:"relation,omitempty"`
	ObjectName  string `json:"object_name,omitempty"`
}

type AISignalRelatedReasoning struct {
	SignalID     string               `json:"signal_id,omitempty"`
	AssetCode    string               `json:"asset_code,omitempty"`
	AssetName    string               `json:"asset_name,omitempty"`
	Description  []string             `json:"description"`
	Relationship AISignalRelationship `json:"relationship"`
	Stocks       []RelatedStock       `json:"stocks"`
}

type AISignalTerms struct {
	ServiceAgreed             bool `json:"service_agreed"`
	PersonalizedServiceAgreed bool `json:"personalized_service_agreed"`
}

// AISignalDetail is the full reasoning page for a product. Found=false is a
// normal, explicit state: a valid product can have no current signal.
type AISignalDetail struct {
	ProductCode         string                     `json:"product_code"`
	ProductType         string                     `json:"product_type"`
	Found               bool                       `json:"found"`
	SignalID            string                     `json:"signal_id,omitempty"`
	TraceID             string                     `json:"trace_id,omitempty"`
	CreatedAt           string                     `json:"created_at,omitempty"`
	SignalDirection     int                        `json:"signal_direction,omitempty"`
	HasRelatedReasoning bool                       `json:"has_related_reasoning"`
	Description         string                     `json:"description,omitempty"`
	Issue               AISignalIssue              `json:"issue"`
	Keywords            []string                   `json:"keywords"`
	News                []BriefingNews             `json:"news"`
	RelatedCallout      string                     `json:"related_callout,omitempty"`
	Related             []AISignalRelatedReasoning `json:"related"`
	Terms               AISignalTerms              `json:"terms"`
	FetchedAt           time.Time                  `json:"fetched_at"`
}
