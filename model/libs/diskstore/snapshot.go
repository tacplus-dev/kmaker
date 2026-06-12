// Copyright (c) 2024 Second Star Cavalier, LLC.

package diskstore

// UC1 — atomic typed snapshot (whole object).
//
// These functions operate on raw bytes. The per-type generated code produces
// the bytes via the generated ToJSON helper on write, and parses them via
// FromJSON + Validate on read — keeping serialization consistent with the rest
// of tacplus and validation always-on.

import (
	"errors"
	"io/fs"
	"os"
)

// SnapshotOpts holds options for a snapshot write.
type SnapshotOpts struct {
	// Codec is the at-rest compression for the snapshot file.
	Codec Codec
	// Mode is the Unix file mode for the data file (e.g. 0o600). 0 leaves the
	// default.
	Mode os.FileMode
	// ArchiveCount is the number of rotated archives to retain.
	ArchiveCount uint32
}

// DefaultSnapshotOpts returns the default snapshot options (no compression,
// mode 0o600, keep 3 archives).
func DefaultSnapshotOpts() SnapshotOpts {
	return SnapshotOpts{Codec: CodecNone, Mode: 0o600, ArchiveCount: 3}
}

// WriteSnapshot writes a whole-object snapshot atomically.
//
// Order is chosen so the active file is only briefly absent (one rename apart):
//  1. compress the payload,
//  2. write it to a fresh fsync'd temp (active still intact),
//  3. rotate the previous active into <path>.bak.1 (keep ArchiveCount),
//  4. atomically rename the temp over path.
func WriteSnapshot(path string, data []byte, opts SnapshotOpts) error {
	payload, err := Compress(data, opts.Codec)
	if err != nil {
		return err
	}
	tmp, err := writeTemp(path, payload, opts.Mode)
	if err != nil {
		return err
	}
	if err := rotateArchives(path, opts.ArchiveCount); err != nil {
		return err
	}
	return promote(tmp, path)
}

// ReadSnapshot reads a snapshot, auto-decompressing. The bool is false when the
// file is absent. The caller deserializes + validates the returned bytes
// (FromJSON + Validate).
func ReadSnapshot(path string) ([]byte, bool, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, wrapIOError(err)
	}
	out, err := DecompressAuto(bytes)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}
