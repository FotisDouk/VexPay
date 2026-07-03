package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// createInvoiceRequest is the POST /v1/invoices body.
type createInvoiceRequest struct {
	Chain  string `json:"chain"`
	Asset  string `json:"asset"`
	Amount string `json:"amount"`

	Wallet struct {
		XPub           string `json:"xpub"`
		Address        string `json:"address"`
		ViewKey        string `json:"view_key"`
		PrimaryAddress string `json:"primary_address"`
	} `json:"wallet"`

	FiatCurrency string            `json:"fiat_currency"`
	FiatAmount   string            `json:"fiat_amount"`
	Rate         string            `json:"rate"`
	Metadata     map[string]string `json:"metadata"`
}

func (s *Server) handleInvoicesCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createInvoice(w, r)
	case http.MethodGet:
		s.listInvoices(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		methodNotAllowed(w, "GET, POST")
	}
}

func (s *Server) createInvoice(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())

	var req createInvoiceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if req.Chain == "" {
		writeError(w, http.StatusBadRequest, "chain is required")
		return
	}
	if req.Asset == "" {
		writeError(w, http.StatusBadRequest, "asset is required")
		return
	}
	asset, err := money.Lookup(req.Asset)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	amount, err := money.ParseDecimal(asset, req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid amount: "+err.Error())
		return
	}

	inv, err := s.deps.Invoices.Create(r.Context(), invoice.CreateParams{
		MerchantID: principal.MerchantID,
		Chain:      chain.ID(req.Chain),
		Wallet: chain.WalletConfig{
			XPub:           req.Wallet.XPub,
			Address:        req.Wallet.Address,
			ViewKey:        req.Wallet.ViewKey,
			PrimaryAddress: req.Wallet.PrimaryAddress,
		},
		Amount:       amount,
		FiatCurrency: req.FiatCurrency,
		FiatAmount:   req.FiatAmount,
		Rate:         req.Rate,
		Metadata:     req.Metadata,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, invoice.NewView(inv))
}

func (s *Server) listInvoices(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFrom(r.Context())

	limit := queryInt(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := queryInt(r, "offset", 0)

	invs, err := s.deps.Invoices.List(r.Context(), principal.MerchantID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list invoices")
		return
	}
	views := make([]invoice.View, len(invs))
	for i, inv := range invs {
		views[i] = invoice.NewView(inv)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": views})
}

func (s *Server) handleInvoiceItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	principal, _ := principalFrom(r.Context())
	id := strings.TrimPrefix(r.URL.Path, "/v1/invoices/")
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
	// Never leak another merchant's invoice.
	if inv.MerchantID != principal.MerchantID {
		writeError(w, http.StatusNotFound, "invoice not found")
		return
	}
	writeJSON(w, http.StatusOK, invoice.NewView(inv))
}

// decodeJSON strictly decodes a JSON request body, writing a 400 on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return err
	}
	return nil
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
