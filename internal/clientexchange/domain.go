// Package clientexchange implements the independent Client Exchange domain.
// It intentionally does not import or reference vacancy/resume dictionaries.
package clientexchange

import "time"

var DictionaryKinds = []string{
	"employee_range", "industry", "marketplace", "accounting_state",
	"transfer_reason", "edo_provider", "transfer_type", "tax_system",
	"revenue_range", "accounting_program",
}

type DictionaryItem struct {
	ID           int64    `json:"id"`
	Kind         string   `json:"kind"`
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	MinValue     *float64 `json:"min,omitempty"`
	MaxValue     *float64 `json:"max,omitempty"`
	Color        string   `json:"color"`
	Icon         string   `json:"icon"`
	LegalName    string   `json:"legal_name,omitempty"`
	OperatorCode string   `json:"operator_code,omitempty"`
	SortOrder    int      `json:"sort_order"`
	Active       bool     `json:"active"`
	Used         bool     `json:"used,omitempty"`
}

type ListingInput struct {
	Title                 string   `json:"title"`
	ClientINN             string   `json:"client_inn"`
	ClientLegalName       string   `json:"client_legal_name"`
	IndustryID            *int64   `json:"industry_id"`
	IndustryIDs           []int64  `json:"industry_ids"`
	EmployeeRangeID       *int64   `json:"employee_range_id"`
	TaxSystemID           *int64   `json:"tax_system_id"`
	RevenueRangeID        *int64   `json:"revenue_range_id"`
	AccountingStateID     *int64   `json:"accounting_state_id"`
	TransferReasonID      *int64   `json:"transfer_reason_id"`
	TransferReasonIDs     []int64  `json:"transfer_reason_ids"`
	TransferTypeID        *int64   `json:"transfer_type_id"`
	TransferReasonComment string   `json:"transfer_reason_comment"`
	TransferPrice         *float64 `json:"transfer_price"`
	MonthlyCommission     *float64 `json:"monthly_commission_percent"`
	CommissionMonths      *int     `json:"commission_months"`
	CurrentMonthlyFee     *float64 `json:"current_monthly_fee"`
	OperationsPerMonth    *int     `json:"operations_per_month"`
	BanksCount            *int     `json:"banks_count"`
	HasVAT                bool     `json:"has_vat"`
	ForeignTrade          bool     `json:"foreign_trade"`
	BargainAllowed        bool     `json:"bargain_allowed"`
	Region                string   `json:"region"`
	City                  string   `json:"city"`
	ClientSince           *string  `json:"client_since"`
	DesiredTransferDate   *string  `json:"desired_transfer_date"`
	Comment               string   `json:"comment"`
	CurrentStep           int      `json:"current_step"`
	MarketplaceIDs        []int64  `json:"marketplace_ids"`
	EDOProviderIDs        []int64  `json:"edo_provider_ids"`
	AccountingProgramIDs  []int64  `json:"accounting_program_ids"`
}

type ResponseInput struct {
	ProposedPrice       *float64 `json:"proposed_price"`
	AcceptOriginalPrice bool     `json:"accept_original_price"`
	ReadyToDiscuss      bool     `json:"ready_to_discuss"`
	Comment             string   `json:"comment"`
}

type UserIdentity struct {
	ID       int64
	FullName string
	Email    string
	Avatar   string
}

type Notification struct {
	ID         int64      `json:"id"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	ListingID  *int64     `json:"listing_id,omitempty"`
	ResponseID *int64     `json:"response_id,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
