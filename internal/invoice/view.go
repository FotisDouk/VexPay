package invoice

import "time"

// View is the JSON representation of an invoice returned by the API and embedded
// in webhook payloads. Amounts are decimal strings so no precision is lost by
// the client.
type View struct {
	ID         string `json:"id"`
	MerchantID string `json:"merchant_id"`

	Chain  string `json:"chain"`
	Asset  string `json:"asset"`
	Amount string `json:"amount"`

	Status                string `json:"status"`
	Received              string `json:"received"`
	Confirmations         int    `json:"confirmations"`
	RequiredConfirmations int    `json:"required_confirmations"`

	ReceiveAddress string `json:"receive_address"`
	PaymentURI     string `json:"payment_uri"`
	TxHash         string `json:"tx_hash,omitempty"`

	FiatCurrency string `json:"fiat_currency,omitempty"`
	FiatAmount   string `json:"fiat_amount,omitempty"`
	Rate         string `json:"rate,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NewView builds a View from an Invoice.
func NewView(inv *Invoice) View {
	return View{
		ID:                    inv.ID,
		MerchantID:            inv.MerchantID,
		Chain:                 string(inv.Chain),
		Asset:                 inv.Asset.Symbol,
		Amount:                inv.Amount.String(),
		Status:                string(inv.Status),
		Received:              inv.Received.String(),
		Confirmations:         inv.Confirmations,
		RequiredConfirmations: inv.RequiredConfs,
		ReceiveAddress:        inv.ReceiveAddress,
		PaymentURI:            inv.PaymentURI,
		TxHash:                inv.TxHash,
		FiatCurrency:          inv.FiatCurrency,
		FiatAmount:            inv.FiatAmount,
		Rate:                  inv.Rate,
		Metadata:              inv.Metadata,
		CreatedAt:             inv.CreatedAt,
		ExpiresAt:             inv.ExpiresAt,
		PaidAt:                inv.PaidAt,
		UpdatedAt:             inv.UpdatedAt,
	}
}
