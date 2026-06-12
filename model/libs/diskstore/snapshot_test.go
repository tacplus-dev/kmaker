// Copyright (c) 2024 Second Star Cavalier, LLC.

package diskstore

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSnapshotRoundtripAndArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	opts := SnapshotOpts{Codec: CodecGz, Mode: 0o600, ArchiveCount: 2}

	if err := WriteSnapshot(path, []byte("v1"), opts); err != nil {
		t.Fatalf("write v1: %v", err)
	}
	got, ok, err := ReadSnapshot(path)
	if err != nil || !ok {
		t.Fatalf("read v1: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(got, []byte("v1")) {
		t.Fatalf("got %q want v1", got)
	}

	if err := WriteSnapshot(path, []byte("v2"), opts); err != nil {
		t.Fatalf("write v2: %v", err)
	}
	got, ok, err = ReadSnapshot(path)
	if err != nil || !ok {
		t.Fatalf("read v2: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(got, []byte("v2")) {
		t.Fatalf("got %q want v2", got)
	}
	if !fileExists(filepath.Join(dir, "state.json.bak.1")) {
		t.Fatalf("previous active should be archived")
	}
}

func TestReadAbsentIsNotOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	_, ok, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("read absent: %v", err)
	}
	if ok {
		t.Fatalf("absent snapshot should report ok=false")
	}
}
