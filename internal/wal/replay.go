package wal

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// maxLineSize bounds a single WAL record line; generous for the small,
// fixed-shape records this log stores.
const maxLineSize = 1 << 20 // 1MiB

// ErrIntegrityViolation is returned by Replay when a record's MAC doesn't
// match what's expected given the signing key and the chain built from
// every record before it — i.e. the record was edited, inserted, deleted,
// or reordered by something other than Append. Unlike a truncated trailing
// line (tolerated as a crash artifact), this is never tolerated: a
// syntactically well-formed record with a wrong MAC is exactly what a
// hand-edited or forged record looks like, so Replay stops immediately
// rather than reconstructing state from it.
var ErrIntegrityViolation = errors.New("wal: record failed integrity check")

// Replay reads every Record from the WAL file at path, in order, invoking
// apply for each one. If path does not exist yet, Replay returns (0, nil,
// false, nil): a fresh exchange with no history to recover.
//
// If key is non-empty, every record's MAC is verified against the hash
// chain built from every record before it (see mac.go); a mismatch returns
// ErrIntegrityViolation immediately, without calling apply for that record
// or any after it. If key is empty, MACs are not checked at all — this
// matches a WAL that was never signed, but also means an already-signed
// WAL's history isn't verified if the key is later omitted; that's a
// deliberate, explicit operator choice, not a silent gap. lastMAC is the
// chain state after the last successfully verified record, to hand to Open
// so future Appends continue the same chain.
//
// If the very last line in the file fails to decode as JSON at all, it is
// treated as a partial write left behind by a crash mid-fsync: Replay
// stops, reports it via the truncated return value, and does not error. A
// decode failure on any earlier line is real corruption and is returned as
// an error.
func Replay(path string, key []byte, apply func(Record) error) (lastSeq uint64, lastMAC []byte, truncated bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil, false, nil
		}
		return 0, nil, false, fmt.Errorf("wal: open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lastSeq, lastMAC, false, fmt.Errorf("wal: read %s: %w", path, err)
	}

	for i, line := range lines {
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			if i == len(lines)-1 {
				return lastSeq, lastMAC, true, nil
			}
			return lastSeq, lastMAC, false, fmt.Errorf("wal: corrupt record at line %d of %s: %w", i+1, path, err)
		}

		if len(key) > 0 {
			if !verifyMAC(key, lastMAC, rec) {
				return lastSeq, lastMAC, false, fmt.Errorf("%w: seq=%d line=%d of %s", ErrIntegrityViolation, rec.Seq, i+1, path)
			}
			// Already validated as decodable by verifyMAC above.
			lastMAC, _ = hex.DecodeString(rec.MAC)
		}

		if err := apply(rec); err != nil {
			return lastSeq, lastMAC, false, fmt.Errorf("wal: apply record seq=%d: %w", rec.Seq, err)
		}
		lastSeq = rec.Seq
	}

	return lastSeq, lastMAC, false, nil
}
