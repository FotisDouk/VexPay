package btc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// Official BIP84 test vector: the account-level zpub for the "abandon ... about"
// mnemonic, with its first two receive addresses (m/84'/0'/0'/0/0 and .../0/1).
const bip84Zpub = "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs"

func TestDeriveMatchesBIP84Vector(t *testing.T) {
	a := New(Mainnet, nil, 2)
	amount, _ := money.ParseDecimal(money.BTC, "0.01")

	cases := []struct {
		index uint32
		want  string
	}{
		{0, "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"},
		{1, "bc1qnjg0jd8228aq7egyzacy8cys3knf9xvrerkf9g"},
	}
	for _, c := range cases {
		target, err := a.DeriveReceiveTarget(chain.WalletConfig{XPub: bip84Zpub}, c.index, amount)
		if err != nil {
			t.Fatalf("derive index %d: %v", c.index, err)
		}
		if target.Address != c.want {
			t.Errorf("index %d address = %s, want %s", c.index, target.Address, c.want)
		}
		if target.Strategy != chain.StrategyAddressPerInvoice {
			t.Errorf("index %d strategy = %s", c.index, target.Strategy)
		}
	}
}

func TestDeriveRejectsMissingXPub(t *testing.T) {
	a := New(Mainnet, nil, 2)
	amount := money.Zero(money.BTC)
	if _, err := a.DeriveReceiveTarget(chain.WalletConfig{}, 0, amount); err == nil {
		t.Fatal("expected error when xpub is missing")
	}
}

func TestPaymentURIIsBIP21(t *testing.T) {
	a := New(Mainnet, nil, 2)
	amount, _ := money.ParseDecimal(money.BTC, "0.0025")
	target, err := a.DeriveReceiveTarget(chain.WalletConfig{XPub: bip84Zpub}, 0, amount)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	uri := a.PaymentURI(target)
	if !strings.HasPrefix(uri, "bitcoin:bc1q") || !strings.Contains(uri, "amount=0.00250000") {
		t.Errorf("unexpected BIP21 URI: %s", uri)
	}
}

func TestCheckPaymentReadsBackend(t *testing.T) {
	const addr = "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/address/"+addr:
			// 250000 sat confirmed, nothing in mempool.
			_, _ = w.Write([]byte(`{"chain_stats":{"funded_txo_sum":250000},"mempool_stats":{"funded_txo_sum":0}}`))
		case r.URL.Path == "/api/blocks/tip/height":
			_, _ = w.Write([]byte(`105`))
		case r.URL.Path == "/api/address/"+addr+"/txs":
			_, _ = w.Write([]byte(`[{"txid":"abcd","vout":[{"scriptpubkey_address":"` + addr + `"}],"status":{"confirmed":true,"block_height":103}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := New(Mainnet, NewMempoolBackend(srv.URL), 2)
	obs, err := a.CheckPayment(context.Background(), chain.ReceiveTarget{Address: addr})
	if err != nil {
		t.Fatalf("check payment: %v", err)
	}
	if !obs.Seen {
		t.Fatal("expected payment to be seen")
	}
	if obs.Received.String() != "0.00250000" {
		t.Errorf("received = %s, want 0.00250000", obs.Received)
	}
	if obs.Confirmations != 3 { // 105 - 103 + 1
		t.Errorf("confirmations = %d, want 3", obs.Confirmations)
	}
	if obs.TxHash != "abcd" {
		t.Errorf("txhash = %s, want abcd", obs.TxHash)
	}
}
