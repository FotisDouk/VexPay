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
	"github.com/vexarnetwork/vexpay/internal/qr"
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

	params := invoice.CreateParams{
		MerchantID: principal.MerchantID,
		Chain:      chain.ID(req.Chain),
		Wallet: chain.WalletConfig{
			XPub:           req.Wallet.XPub,
			Address:        req.Wallet.Address,
			ViewKey:        req.Wallet.ViewKey,
			PrimaryAddress: req.Wallet.PrimaryAddress,
		},
		FiatCurrency: req.FiatCurrency,
		FiatAmount:   req.FiatAmount,
		Rate:         req.Rate,
		Metadata:     req.Metadata,
	}

	// Crypto-priced: an explicit amount (with its asset). Otherwise the service
	// falls back to fiat pricing from fiat_currency + fiat_amount.
	if req.Amount != "" {
		if req.Asset == "" {
			writeError(w, http.StatusBadRequest, "asset is required when amount is set")
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
		params.Amount = amount
	}

	inv, err := s.deps.Invoices.Create(r.Context(), params)
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

	rest := strings.TrimPrefix(r.URL.Path, "/v1/invoices/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
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

	switch {
	case len(parts) == 1:
		writeJSON(w, http.StatusOK, invoice.NewView(inv))
	case len(parts) == 2 && parts[1] == "qr":
		s.writeInvoiceQR(w, r, inv)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// writeInvoiceQR renders the invoice's payment URI as a PNG QR code.
func (s *Server) writeInvoiceQR(w http.ResponseWriter, r *http.Request, inv *invoice.Invoice) {
	png, err := qr.PNG(inv.PaymentURI, queryInt(r, "size", 256))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render QR code")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
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
