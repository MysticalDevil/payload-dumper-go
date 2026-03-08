package main

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valyala/gozstd"
	"github.com/vbauerster/mpb/v5"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestExtractZeroOperation(t *testing.T) {
	p := newExtractPayload(t, nil)
	defer p.file.Close()

	out := createTempImage(t)
	defer out.Close()

	part := extractPartitionWithOperation(chromeos_update_engine.InstallOperation_ZERO, nil, nil, oneBlockExtent())
	if err := p.Extract(part, out); err != nil {
		t.Fatalf("extract ZERO operation: %v", err)
	}

	got, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatalf("read output image: %v", err)
	}
	if len(got) != blockSize {
		t.Fatalf("unexpected output size: got %d, want %d", len(got), blockSize)
	}
	for _, b := range got {
		if b != 0 {
			t.Fatal("expected zero-filled output")
		}
	}
}

func TestExtractZstdOperation(t *testing.T) {
	plain := bytesOf('Z', blockSize)
	compressed := gozstd.Compress(nil, plain)
	hash := sha256.Sum256(compressed)

	p := newExtractPayload(t, compressed)
	defer p.file.Close()

	out := createTempImage(t)
	defer out.Close()

	part := extractPartitionWithOperation(
		chromeos_update_engine.InstallOperation_ZSTD,
		compressed,
		hash[:],
		oneBlockExtent(),
	)
	if err := p.Extract(part, out); err != nil {
		t.Fatalf("extract ZSTD operation: %v", err)
	}

	got, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatalf("read output image: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatal("unexpected ZSTD extracted content")
	}
}

func TestExtractInvalidDstExtents(t *testing.T) {
	p := newExtractPayload(t, bytesOf('R', blockSize))
	defer p.file.Close()
	out := createTempImage(t)
	defer out.Close()

	part := extractPartitionWithOperation(chromeos_update_engine.InstallOperation_REPLACE, bytesOf('R', blockSize), nil, nil)
	err := p.Extract(part, out)
	if err == nil || !strings.Contains(err.Error(), "Invalid operation.DstExtents") {
		t.Fatalf("expected invalid extents error, got %v", err)
	}
}

func TestExtractUnhandledOperationType(t *testing.T) {
	p := newExtractPayload(t, nil)
	defer p.file.Close()
	out := createTempImage(t)
	defer out.Close()

	part := extractPartitionWithOperation(chromeos_update_engine.InstallOperation_MOVE, nil, nil, oneBlockExtent())
	err := p.Extract(part, out)
	if err == nil || !strings.Contains(err.Error(), "Unhandled operation type") {
		t.Fatalf("expected unhandled type error, got %v", err)
	}
}

func newExtractPayload(t *testing.T, blob []byte) *Payload {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blob.bin")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open blob: %v", err)
	}

	return &Payload{
		file:       f,
		dataOffset: 0,
		progress:   mpb.New(),
	}
}

func createTempImage(t *testing.T) *os.File {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "img_*.bin")
	if err != nil {
		t.Fatalf("create output image: %v", err)
	}

	return out
}

func oneBlockExtent() []*chromeos_update_engine.Extent {
	start := uint64(0)
	num := uint64(1)
	return []*chromeos_update_engine.Extent{
		{
			StartBlock: &start,
			NumBlocks:  &num,
		},
	}
}

func extractPartitionWithOperation(opType chromeos_update_engine.InstallOperation_Type, data []byte, hash []byte, extents []*chromeos_update_engine.Extent) *chromeos_update_engine.PartitionUpdate {
	name := "test"
	size := uint64(blockSize)
	dataOffset := uint64(0)
	dataLen := uint64(len(data))

	return &chromeos_update_engine.PartitionUpdate{
		PartitionName: &name,
		NewPartitionInfo: &chromeos_update_engine.PartitionInfo{
			Size: &size,
		},
		Operations: []*chromeos_update_engine.InstallOperation{
			{
				Type:           &opType,
				DataOffset:     &dataOffset,
				DataLength:     &dataLen,
				DstExtents:     extents,
				DataSha256Hash: hash,
			},
		},
	}
}
