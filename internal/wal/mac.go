package wal

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// computeMAC returns the HMAC-SHA256 of prevMAC concatenated with rec's
// signable fields (everything except MAC itself), keyed by key. Each
// record's MAC therefore depends on every record before it — a hash chain —
// so modifying, deleting, reordering, or inserting any record changes the
// MAC of every record from that point forward. Fields are serialized with
// explicit delimiters (not JSON) so the exact byte layout being signed is
// obvious from reading this function, rather than depending on
// encoding/json's field-ordering behavior.
func computeMAC(key, prevMAC []byte, rec Record) []byte {
	var buf bytes.Buffer
	buf.Write(prevMAC)
	buf.WriteByte('\x00')
	fmt.Fprintf(&buf, "%d\x00%d\x00%s\x00%s\x00%d\x00%s\x00%t\x00%d\x00%d\x00%d\x00%d\x00%s",
		rec.Seq, rec.Timestamp, rec.Symbol, rec.Action, rec.OrderID, rec.Maker, rec.IsBid,
		rec.Size, rec.Price, rec.Nonce, rec.Expiry, rec.Signature)

	mac := hmac.New(sha256.New, key)
	mac.Write(buf.Bytes())
	return mac.Sum(nil)
}

// verifyMAC reports whether rec's stored hex MAC is the correct HMAC for
// rec chained from prevMAC. It never panics on malformed input — an
// unparseable MAC simply fails verification, since that's exactly what a
// hand-edited record looks like. Uses a constant-time comparison so a
// verifier can't be timing-attacked into leaking the expected value.
func verifyMAC(key, prevMAC []byte, rec Record) bool {
	got, err := hex.DecodeString(rec.MAC)
	if err != nil {
		return false
	}
	want := computeMAC(key, prevMAC, rec)
	return subtle.ConstantTimeCompare(got, want) == 1
}
