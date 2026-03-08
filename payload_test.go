package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestSetConcurrencyRejectsInvalidValue(t *testing.T) {
	p := NewPayload("dummy")

	if err := p.SetConcurrency(0); err == nil {
		t.Fatal("expected error for invalid concurrency")
	}

	if got := p.GetConcurrency(); got != 4 {
		t.Fatalf("concurrency changed on invalid value: got %d, want 4", got)
	}
}

func TestExtractSelectedReturnsWorkerOpenError(t *testing.T) {
	name := "boot"
	p := &Payload{
		initialized: true,
		concurrency: 1,
		progress:    nil,
		requests:    nil,
		deltaArchiveManifest: &chromeos_update_engine.DeltaArchiveManifest{
			Partitions: []*chromeos_update_engine.PartitionUpdate{
				{PartitionName: &name},
			},
		},
	}

	err := p.ExtractSelected(filepath.Join(t.TempDir(), "missing-dir"), []string{"boot"})
	if err == nil {
		t.Fatal("expected extraction error for missing output directory")
	}
}

func TestCopyToExtentsWritesMultipleRegions(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.img")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output file: %v", err)
	}
	defer out.Close()

	src := bytes.NewReader(append(bytes.Repeat([]byte{'A'}, blockSize), bytes.Repeat([]byte{'B'}, blockSize)...))
	extents := []*chromeos_update_engine.Extent{
		{
			StartBlock: ptrUint64(0),
			NumBlocks:  ptrUint64(1),
		},
		{
			StartBlock: ptrUint64(2),
			NumBlocks:  ptrUint64(1),
		},
	}

	n, err := copyToExtents(out, extents, src)
	if err != nil {
		t.Fatalf("copyToExtents failed: %v", err)
	}
	if n != 2*blockSize {
		t.Fatalf("unexpected bytes written: got %d, want %d", n, 2*blockSize)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) != 3*blockSize {
		t.Fatalf("unexpected output size: got %d, want %d", len(got), 3*blockSize)
	}

	if !bytes.Equal(got[0:blockSize], bytes.Repeat([]byte{'A'}, blockSize)) {
		t.Fatal("first extent content mismatch")
	}
	if !bytes.Equal(got[blockSize:2*blockSize], bytes.Repeat([]byte{0}, blockSize)) {
		t.Fatal("gap extent should remain zeroed")
	}
	if !bytes.Equal(got[2*blockSize:3*blockSize], bytes.Repeat([]byte{'B'}, blockSize)) {
		t.Fatal("second extent content mismatch")
	}
}

func TestExtractSelectedUsesBinarySearchForPartitionFilter(t *testing.T) {
	nameA := "boot"
	nameB := "vendor"
	p := &Payload{
		initialized: true,
		concurrency: 1,
		deltaArchiveManifest: &chromeos_update_engine.DeltaArchiveManifest{
			Partitions: []*chromeos_update_engine.PartitionUpdate{
				{PartitionName: &nameA},
				{PartitionName: &nameB},
			},
		},
	}

	err := p.ExtractSelected(filepath.Join(t.TempDir(), "missing"), []string{"vendor"})
	if err == nil {
		t.Fatal("expected extraction error for selected partition")
	}

	if !strings.Contains(err.Error(), "vendor.img") || strings.Contains(err.Error(), "boot.img") {
		t.Fatalf("expected only selected partition to be processed, got error: %v", err)
	}
}

func ptrUint64(v uint64) *uint64 {
	return &v
}
