// Copyright (c) 2024 Second Star Cavalier, LLC.

package diskstore

// Atomic file write + numbered archive rotation, shared by the snapshot (UC1)
// and the redo-log roll (UC2).
//
// Atomicity model: build a fresh sibling temp file, fsync it, then rename it
// over the target. rename(2) is atomic on a POSIX filesystem, so the target is
// never observed half-written.

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeTemp writes data to a fresh sibling temp file (fsync'd, mode applied) and
// returns its path. The temp is in the same directory as path so the later
// promote rename stays on one filesystem (and is therefore atomic). A mode of 0
// leaves the default permissions.
func writeTemp(path string, data []byte, mode os.FileMode) (string, error) {
	if path == "" {
		return "", ErrInvalidData("path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", wrapIOError(err)
	}
	tmp := sibling(path, fmt.Sprintf(".tmp.%d", os.Getpid()))

	f, err := os.Create(tmp)
	if err != nil {
		return "", wrapIOError(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", wrapIOError(err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", wrapIOError(err)
	}
	if mode != 0 {
		if err := f.Chmod(mode); err != nil {
			f.Close()
			return "", wrapIOError(err)
		}
	}
	if err := f.Close(); err != nil {
		return "", wrapIOError(err)
	}
	return tmp, nil
}

// promote atomically moves tmp onto path (the commit point), then fsyncs the
// parent directory so the rename is durable.
func promote(tmp, path string) error {
	if err := os.Rename(tmp, path); err != nil {
		return wrapIOError(err)
	}
	syncDir(filepath.Dir(path))
	return nil
}

// atomicWrite is writeTemp + promote for callers that don't archive.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp, err := writeTemp(path, data, mode)
	if err != nil {
		return err
	}
	return promote(tmp, path)
}

// rotateArchives rotates numbered archives, then moves the current path to
// <path>.bak.1, retaining count archives. count == 0 simply removes path.
//
// Leaves path absent on return; the caller is expected to immediately promote a
// freshly written temp into path.
func rotateArchives(path string, count uint32) error {
	if count == 0 {
		_ = os.Remove(path)
		syncDir(filepath.Dir(path))
		return nil
	}
	// Drop the oldest, then shift each archive up by one. Iterate descending
	// without relying on uint32 wraparound at zero.
	_ = os.Remove(archiveName(path, count))
	for k := count - 1; k >= 1; k-- {
		from := archiveName(path, k)
		if fileExists(from) {
			if err := os.Rename(from, archiveName(path, k+1)); err != nil {
				return wrapIOError(err)
			}
		}
	}
	if fileExists(path) {
		if err := os.Rename(path, archiveName(path, 1)); err != nil {
			return wrapIOError(err)
		}
	}
	syncDir(filepath.Dir(path))
	return nil
}

func archiveName(path string, n uint32) string {
	return sibling(path, fmt.Sprintf(".bak.%d", n))
}

func sibling(path, suffix string) string {
	return path + suffix
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// syncDir fsyncs a directory so a contained rename is durable. Errors are
// best-effort (matches the Rust sync_dir behavior).
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
