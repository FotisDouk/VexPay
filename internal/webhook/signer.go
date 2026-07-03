// Package webhook signs and delivers invoice status-change events to merchant
// endpoints. Deliveries are authenticated with an HMAC-SHA256 signature and a
// timestamp so recipients can verify authenticity and reject replays.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader is the HTTP header carrying the signature.
const SignatureHeader = "VexPay-Signature"

// Sign returns the header value for a payload signed at time ts.
// Format: "t=<unix>,v1=<hex-hmac>", where the HMAC is computed over
// "<unix>.<body>".
func Sign(secret string, ts time.Time, body []byte) string {
	unix := ts.Unix()
	mac := computeMAC(secret, unix, body)
	return fmt.Sprintf("t=%d,v1=%s", unix, mac)
}

// Verify checks a signature header against the body using secret, rejecting
// signatures whose timestamp is outside tolerance (replay protection).
func Verify(secret, header string, body []byte, tolerance time.Duration) error {
	var tsStr, sig string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			tsStr = v
		case "v1":
			sig = v
		}
	}
	if tsStr == "" || sig == "" {
		return fmt.Errorf("malformed signature header")
	}
	unix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if tolerance > 0 {
		age := time.Since(time.Unix(unix, 0))
		if age < 0 {
			age = -age
		}
		if age > tolerance {
			return fmt.Errorf("signature timestamp outside tolerance")
		}
	}
	expected := computeMAC(secret, unix, body)
	// Constant-time comparison.
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func computeMAC(secret string, unix int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.", unix)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
