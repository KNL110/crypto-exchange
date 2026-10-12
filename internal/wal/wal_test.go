package wal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := Open(path, 1, nil, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	want := []Record{
		{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100},
		{Symbol: "BTC-USD", Action: ActionPlaceMarket, OrderID: 2, IsBid: false, Size: 5},
		{Symbol: "BTC-USD", Action: ActionCancel, OrderID: 1},
	}
	for i := range want {
		rec, err := w.Append(want[i])
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
		want[i] = rec
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var got []Record
	lastSeq, _, truncated, err := Replay(path, nil, func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if truncated {
		t.Fatalf("Replay reported truncated on a clean file")
	}
	if lastSeq != 3 {
		t.Fatalf("lastSeq = %d, want 3", lastSeq)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestReplayNonexistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.log")

	lastSeq, lastMAC, truncated, err := Replay(path, nil, func(Record) error { return nil })
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if truncated {
		t.Fatalf("truncated = true for a missing file, want false")
	}
	if lastSeq != 0 {
		t.Fatalf("lastSeq = %d, want 0", lastSeq)
	}
	if lastMAC != nil {
		t.Fatalf("lastMAC = %v, want nil", lastMAC)
	}
}

func TestReplayTruncatedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := Open(path, 1, nil, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate a crash mid-write: append a partial, invalid JSON line.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, err := f.WriteString(`{"seq":2,"symbol":"BTC-USD","action":"place_l`); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var got []Record
	lastSeq, _, truncated, err := Replay(path, nil, func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if !truncated {
		t.Fatalf("truncated = false, want true")
	}
	if lastSeq != 1 {
		t.Fatalf("lastSeq = %d, want 1", lastSeq)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
}

func TestReplayCorruptMiddleRecordErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := Open(path, 1, nil, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	// A corrupt line that is NOT the last line in the file.
	if _, err := f.WriteString("not json at all\n"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if _, err := f.WriteString(`{"seq":3,"symbol":"BTC-USD","action":"cancel","order_id":1}` + "\n"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, _, _, err = Replay(path, nil, func(Record) error { return nil })
	if err == nil {
		t.Fatalf("Replay: expected error for corrupt middle record, got nil")
	}
}

func TestSignedAppendAndReplayRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-signing-key")

	w, err := Open(path, 1, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, rec := range []Record{
		{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100},
		{Symbol: "BTC-USD", Action: ActionCancel, OrderID: 1},
	} {
		if _, err := w.Append(rec); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var got []Record
	lastSeq, lastMAC, truncated, err := Replay(path, key, func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if truncated {
		t.Fatalf("truncated = true, want false")
	}
	if lastSeq != 2 {
		t.Fatalf("lastSeq = %d, want 2", lastSeq)
	}
	if len(lastMAC) == 0 {
		t.Fatalf("lastMAC is empty, want the chain's final MAC")
	}
	for _, r := range got {
		if r.MAC == "" {
			t.Errorf("record seq=%d has no MAC", r.Seq)
		}
	}
}

func TestReplayDetectsEditedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-signing-key")

	w, err := Open(path, 1, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Tamper with the record's size after the fact, keeping its MAC as-is —
	// exactly what hand-editing the file looks like.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	tampered := bytes.Replace(raw, []byte(`"size":10`), []byte(`"size":999999`), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatalf("test setup: tamper replacement did not match anything")
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, _, err = Replay(path, key, func(Record) error { return nil })
	if !errors.Is(err, ErrIntegrityViolation) {
		t.Fatalf("Replay: err = %v, want ErrIntegrityViolation", err)
	}
}

func TestReplayDetectsInsertedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-signing-key")

	w, err := Open(path, 1, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Hand-craft an extra record the way someone without the key would —
	// no valid MAC, since they can't compute one without it. This is the
	// exact scenario of manually adding duplicate/fabricated lines to the
	// file in an editor.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, err := f.WriteString(`{"seq":2,"symbol":"BTC-USD","action":"place_limit","order_id":1,"is_bid":true,"size":10,"price":100}` + "\n"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var applied int
	_, _, _, err = Replay(path, key, func(Record) error { applied++; return nil })
	if !errors.Is(err, ErrIntegrityViolation) {
		t.Fatalf("Replay: err = %v, want ErrIntegrityViolation", err)
	}
	if applied != 1 {
		t.Fatalf("apply was called %d times, want exactly 1 (only the legitimate record)", applied)
	}
}

func TestReplayWithoutKeySkipsVerification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-signing-key")

	w, err := Open(path, 1, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Replaying a signed file without providing the key is a deliberate,
	// explicit choice to skip verification — it must not error.
	lastSeq, _, _, err := Replay(path, nil, func(Record) error { return nil })
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if lastSeq != 1 {
		t.Fatalf("lastSeq = %d, want 1", lastSeq)
	}
}

func TestReplayRejectsUnsignedRecordsWhenKeyProvided(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	// A WAL written before signing was ever enabled.
	w, err := Open(path, 1, nil, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Turning on WAL_HMAC_KEY against pre-existing unsigned history must
	// fail loudly rather than silently accept it — an attacker could
	// otherwise strip the MAC field to bypass verification.
	_, _, _, err = Replay(path, []byte("a-key-added-later"), func(Record) error { return nil })
	if !errors.Is(err, ErrIntegrityViolation) {
		t.Fatalf("Replay: err = %v, want ErrIntegrityViolation", err)
	}
}

func TestReplayContinuesChainAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	key := []byte("test-signing-key")

	w, err := Open(path, 1, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := w.Append(Record{Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate a restart: replay to resume chain state, reopen, append more.
	lastSeq, lastMAC, _, err := Replay(path, key, func(Record) error { return nil })
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	w2, err := Open(path, lastSeq+1, key, lastMAC)
	if err != nil {
		t.Fatalf("Open (2nd): %v", err)
	}
	if _, err := w2.Append(Record{Symbol: "BTC-USD", Action: ActionCancel, OrderID: 1}); err != nil {
		t.Fatalf("Append (2nd): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (2nd): %v", err)
	}

	// The full file, including the record appended after reopening, must
	// still verify as one unbroken chain.
	var got []Record
	_, _, _, err = Replay(path, key, func(r Record) error { got = append(got, r); return nil })
	if err != nil {
		t.Fatalf("Replay (final): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
}

func TestVerifyMACRejectsMalformedHex(t *testing.T) {
	rec := Record{Seq: 1, Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100, MAC: "not-hex-!!"}
	if verifyMAC([]byte("key"), nil, rec) {
		t.Fatalf("verifyMAC accepted malformed hex")
	}
}

func TestComputeMACIsDeterministicAndKeySensitive(t *testing.T) {
	rec := Record{Seq: 1, Symbol: "BTC-USD", Action: ActionPlaceLimit, OrderID: 1, IsBid: true, Size: 10, Price: 100}

	a := computeMAC([]byte("key-a"), nil, rec)
	b := computeMAC([]byte("key-a"), nil, rec)
	if !bytes.Equal(a, b) {
		t.Fatalf("computeMAC not deterministic for identical inputs")
	}

	c := computeMAC([]byte("key-b"), nil, rec)
	if bytes.Equal(a, c) {
		t.Fatalf("computeMAC produced the same output for different keys")
	}
}

// Guard against a regression where the byte layout in computeMAC no longer
// distinguishes e.g. Symbol "AB" + OrderID from Symbol "A" + a differently
// shaped OrderID — the '\x00' delimiters exist specifically to prevent
// field-concatenation ambiguity.
func TestComputeMACDelimitsFields(t *testing.T) {
	a := computeMAC([]byte("k"), nil, Record{Symbol: "AB", Action: "x"})
	b := computeMAC([]byte("k"), nil, Record{Symbol: "A", Action: "Bx"})
	if bytes.Equal(a, b) {
		t.Fatalf("computeMAC collided across a field boundary")
	}
}

// Every signed-order field is part of the MAC, not just the original ones.
func TestMACCoversSignedOrderFields(t *testing.T) {
	key := []byte("test-signing-key")
	base := Record{
		Seq: 1, Timestamp: 100, Symbol: "ETH", Action: ActionPlaceLimit, OrderID: 1,
		Maker: "0x00000000000000000000000000000000000000aa", IsBid: true, Size: 10, Price: 3_500,
		Nonce: 7, Expiry: 200, Signature: "0x01",
	}
	want := computeMAC(key, nil, base)

	edits := map[string]func(*Record){
		"maker":     func(r *Record) { r.Maker = "0x00000000000000000000000000000000000000bb" },
		"nonce":     func(r *Record) { r.Nonce = 8 },
		"expiry":    func(r *Record) { r.Expiry = 999 },
		"signature": func(r *Record) { r.Signature = "0x02" },
		"price":     func(r *Record) { r.Price = 1 },
	}
	for name, edit := range edits {
		rec := base
		edit(&rec)
		if bytes.Equal(computeMAC(key, nil, rec), want) {
			t.Errorf("editing %s did not change the MAC", name)
		}
	}
}
