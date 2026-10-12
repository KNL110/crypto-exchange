package wal

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// WAL is an append-only write-ahead log backed by a single file on disk.
// Every Append is fsynced before returning, so a Record that was
// successfully appended is guaranteed to survive a crash.
type WAL struct {
	mu      sync.Mutex
	file    *os.File
	nextSeq uint64

	key     []byte // signing key; nil/empty disables integrity checking
	prevMAC []byte // running chain state, resumed from Replay
}

// Open opens (creating if necessary) the WAL file at path for appending,
// creating parent directories as needed.
//
// startSeq is the sequence number to assign to the next appended Record
// (typically lastSeq+1 from a prior Replay, or 1 for a fresh log).
//
// key enables tamper-evident chaining (see mac.go): every Append will sign
// its record, chained from prevMAC (the MAC of the last record already in
// the file, as returned by Replay — nil for a fresh log). A nil/empty key
// disables signing entirely, matching plain append-only behavior with no
// integrity checking.
func Open(path string, startSeq uint64, key, prevMAC []byte) (*WAL, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("wal: create dir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("wal: open %s: %w", path, err)
	}
	if startSeq == 0 {
		startSeq = 1
	}
	return &WAL{file: f, nextSeq: startSeq, key: key, prevMAC: prevMAC}, nil
}

// Append assigns the record the next sequence number, signs it (if a
// signing key was configured via Open) chained from the previous record's
// MAC, writes it as a single JSON line, and fsyncs the file before
// returning. The Seq and MAC fields on rec are overwritten.
func (w *WAL) Append(rec Record) (Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	rec.Seq = w.nextSeq
	rec.MAC = ""

	if len(w.key) > 0 {
		mac := computeMAC(w.key, w.prevMAC, rec)
		rec.MAC = hex.EncodeToString(mac)
		w.prevMAC = mac
	}

	line, err := json.Marshal(rec)
	if err != nil {
		return Record{}, fmt.Errorf("wal: marshal record: %w", err)
	}
	line = append(line, '\n')

	if _, err := w.file.Write(line); err != nil {
		return Record{}, fmt.Errorf("wal: write record: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return Record{}, fmt.Errorf("wal: fsync: %w", err)
	}

	w.nextSeq++
	return rec, nil
}

// Close flushes and closes the underlying file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
