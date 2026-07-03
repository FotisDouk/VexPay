package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (registers "sqlite")

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/money"
)

const schema = `
CREATE TABLE IF NOT EXISTS invoices (
	id               TEXT PRIMARY KEY,
	merchant_id      TEXT NOT NULL,
	chain            TEXT NOT NULL,
	asset_symbol     TEXT NOT NULL,
	asset_decimals   INTEGER NOT NULL,
	amount_units     TEXT NOT NULL,
	fiat_currency    TEXT NOT NULL DEFAULT '',
	fiat_amount      TEXT NOT NULL DEFAULT '',
	rate             TEXT NOT NULL DEFAULT '',
	receive_address  TEXT NOT NULL,
	payment_uri      TEXT NOT NULL,
	strategy         TEXT NOT NULL,
	derivation_index INTEGER NOT NULL,
	status           TEXT NOT NULL,
	received_units   TEXT NOT NULL,
	confirmations    INTEGER NOT NULL,
	required_confs   INTEGER NOT NULL,
	tx_hash          TEXT NOT NULL DEFAULT '',
	metadata         TEXT NOT NULL DEFAULT '{}',
	created_at       TEXT NOT NULL,
	expires_at       TEXT NOT NULL,
	paid_at          TEXT NOT NULL DEFAULT '',
	updated_at       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_invoices_merchant ON invoices(merchant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);

CREATE TABLE IF NOT EXISTS derivation_seq (
	merchant_id TEXT PRIMARY KEY,
	next_index  INTEGER NOT NULL
);
`

type sqliteStore struct {
	db   *sql.DB
	repo *sqliteInvoiceRepo
}

func openSQLite(path string) (Store, error) {
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite serialises writes; a single connection avoids "database is locked".
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	s := &sqliteStore{db: db}
	s.repo = &sqliteInvoiceRepo{db: db}
	return s, nil
}

func (s *sqliteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *sqliteStore) Invoices() invoice.Repository   { return s.repo }
func (s *sqliteStore) Close() error                   { return s.db.Close() }

type sqliteInvoiceRepo struct {
	db *sql.DB
}

const invoiceColumns = `id, merchant_id, chain, asset_symbol, asset_decimals, amount_units,
	fiat_currency, fiat_amount, rate, receive_address, payment_uri, strategy, derivation_index,
	status, received_units, confirmations, required_confs, tx_hash, metadata,
	created_at, expires_at, paid_at, updated_at`

func (r *sqliteInvoiceRepo) Create(ctx context.Context, inv *invoice.Invoice) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO invoices (`+invoiceColumns+`) VALUES
		(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		inv.ID, inv.MerchantID, string(inv.Chain), inv.Asset.Symbol, inv.Asset.Decimals,
		inv.Amount.Units().String(), inv.FiatCurrency, inv.FiatAmount, inv.Rate,
		inv.ReceiveAddress, inv.PaymentURI, string(inv.Strategy), inv.DerivationIndex,
		string(inv.Status), inv.Received.Units().String(), inv.Confirmations, inv.RequiredConfs,
		inv.TxHash, marshalMetadata(inv.Metadata),
		formatTime(inv.CreatedAt), formatTime(inv.ExpiresAt), formatTimePtr(inv.PaidAt), formatTime(inv.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}
	return nil
}

func (r *sqliteInvoiceRepo) Get(ctx context.Context, id string) (*invoice.Invoice, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+invoiceColumns+` FROM invoices WHERE id = ?`, id)
	inv, err := scanInvoice(row)
	if err == sql.ErrNoRows {
		return nil, invoice.ErrNotFound
	}
	return inv, err
}

func (r *sqliteInvoiceRepo) Update(ctx context.Context, inv *invoice.Invoice) error {
	res, err := r.db.ExecContext(ctx, `UPDATE invoices SET
		status=?, received_units=?, confirmations=?, tx_hash=?, paid_at=?, updated_at=?
		WHERE id=?`,
		string(inv.Status), inv.Received.Units().String(), inv.Confirmations, inv.TxHash,
		formatTimePtr(inv.PaidAt), formatTime(inv.UpdatedAt), inv.ID,
	)
	if err != nil {
		return fmt.Errorf("update invoice: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return invoice.ErrNotFound
	}
	return nil
}

func (r *sqliteInvoiceRepo) ListOpen(ctx context.Context) ([]*invoice.Invoice, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+invoiceColumns+` FROM invoices
		WHERE status IN ('pending','underpaid','confirming') ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInvoices(rows)
}

func (r *sqliteInvoiceRepo) ListByMerchant(ctx context.Context, merchantID string, limit, offset int) ([]*invoice.Invoice, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+invoiceColumns+` FROM invoices
		WHERE merchant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, merchantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInvoices(rows)
}

func (r *sqliteInvoiceRepo) NextDerivationIndex(ctx context.Context, merchantID string) (uint32, error) {
	var next int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO derivation_seq (merchant_id, next_index) VALUES (?, 1)
		ON CONFLICT(merchant_id) DO UPDATE SET next_index = next_index + 1
		RETURNING next_index - 1`, merchantID).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("next derivation index: %w", err)
	}
	return uint32(next), nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanInvoice(s scanner) (*invoice.Invoice, error) {
	var (
		inv                                     invoice.Invoice
		chainID, assetSym, strategy, status     string
		amountUnits, receivedUnits, metaJSON    string
		createdAt, expiresAt, paidAt, updatedAt string
		assetDecimals                           int
	)
	if err := s.Scan(
		&inv.ID, &inv.MerchantID, &chainID, &assetSym, &assetDecimals, &amountUnits,
		&inv.FiatCurrency, &inv.FiatAmount, &inv.Rate, &inv.ReceiveAddress, &inv.PaymentURI,
		&strategy, &inv.DerivationIndex, &status, &receivedUnits, &inv.Confirmations,
		&inv.RequiredConfs, &inv.TxHash, &metaJSON, &createdAt, &expiresAt, &paidAt, &updatedAt,
	); err != nil {
		return nil, err
	}

	asset := money.Asset{Symbol: assetSym, Decimals: assetDecimals}
	amount, err := parseAmount(asset, amountUnits)
	if err != nil {
		return nil, err
	}
	received, err := parseAmount(asset, receivedUnits)
	if err != nil {
		return nil, err
	}

	inv.Chain = chain.ID(chainID)
	inv.Asset = asset
	inv.Amount = amount
	inv.Received = received
	inv.Strategy = chain.ReceiveStrategy(strategy)
	inv.Status = invoice.Status(status)
	inv.Metadata = unmarshalMetadata(metaJSON)
	inv.CreatedAt = parseTime(createdAt)
	inv.ExpiresAt = parseTime(expiresAt)
	inv.UpdatedAt = parseTime(updatedAt)
	if paidAt != "" {
		t := parseTime(paidAt)
		inv.PaidAt = &t
	}
	return &inv, nil
}

func scanInvoices(rows *sql.Rows) ([]*invoice.Invoice, error) {
	var out []*invoice.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func parseAmount(asset money.Asset, units string) (money.Amount, error) {
	n, ok := new(big.Int).SetString(units, 10)
	if !ok {
		return money.Amount{}, fmt.Errorf("invalid stored amount %q", units)
	}
	return money.FromUnits(asset, n), nil
}

func marshalMetadata(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func unmarshalMetadata(s string) map[string]string {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
