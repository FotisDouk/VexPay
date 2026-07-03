package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vexarnetwork/vexpay/internal/chain/mock"
	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// sandboxPayRequest is the optional body for the payment simulator. Omitted
// fields default to paying the full amount with enough confirmations to settle.
type sandboxPayRequest struct {
	Amount        string `json:"amount"`
	Confirmations *int   `json:"confirmations"`
	TxHash        string `json:"tx_hash"`
}

// handleSandboxPay simulates an on-chain payment against a mock-chain invoice,
// then advances the invoice so the caller sees the resulting state immediately.
func (s *Server) handleSandboxPay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	principal, _ := principalFrom(r.Context())
	if !principal.Sandbox {
		writeError(w, http.StatusForbidden, "sandbox endpoints require a sandbox API key")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/sandbox/pay/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	inv, err := s.deps.Invoices.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, invoice.ErrNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load invoice")
		return
	}
	if inv.MerchantID != principal.MerchantID {
		writeError(w, http.StatusNotFound, "invoice not found")
		return
	}
	if inv.Chain != mock.ChainID {
		writeError(w, http.StatusBadRequest, "invoice is not on the sandbox chain")
		return
	}

	req := sandboxPayRequest{}
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
	}

	amount := inv.Amount
	if req.Amount != "" {
		amount, err = money.ParseDecimal(inv.Asset, req.Amount)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid amount: "+err.Error())
			return
		}
	}
	confirmations := inv.RequiredConfs
	if req.Confirmations != nil {
		confirmations = *req.Confirmations
	}
	txHash := req.TxHash
	if txHash == "" {
		txHash = "sbxtx_" + inv.ID
	}

	s.deps.Sandbox.Pay(inv.ReceiveAddress, amount, confirmations, txHash)

	if _, err := s.deps.Invoices.Sync(r.Context(), inv); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync invoice")
		return
	}
	writeJSON(w, http.StatusOK, invoice.NewView(inv))
}
