package main

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestPayloadInitAndExtractAllReplace(t *testing.T) {
	block := bytesOf('A', blockSize)
	payloadPath := writeTestPayload(t, testManifestSinglePartition("boot", block, chromeos_update_engine.InstallOperation_REPLACE), nil, block)

	p := NewPayload(payloadPath)
	if err := p.Open(); err != nil {
		t.Fatalf("open payload: %v", err)
	}
	defer p.file.Close()

	if err := p.Init(); err != nil {
		t.Fatalf("init payload: %v", err)
	}
	if err := p.SetConcurrency(1); err != nil {
		t.Fatalf("set concurrency: %v", err)
	}

	outDir := t.TempDir()
	if err := p.ExtractAll(outDir); err != nil {
		t.Fatalf("extract all: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "boot.img"))
	if err != nil {
		t.Fatalf("read extracted image: %v", err)
	}
	if string(got) != string(block) {
		t.Fatal("extracted image content mismatch")
	}

	blob, err := p.readDataBlob(0, int64(len(block)))
	if err != nil {
		t.Fatalf("read data blob: %v", err)
	}
	if string(blob) != string(block) {
		t.Fatal("readDataBlob content mismatch")
	}
}

func TestExtractSelectedOnlyRequestedPartition(t *testing.T) {
	bootData := bytesOf('B', blockSize)
	vendorData := bytesOf('V', blockSize)

	manifest := &chromeos_update_engine.DeltaArchiveManifest{
		Partitions: []*chromeos_update_engine.PartitionUpdate{
			testPartition("boot", bootData, 0, chromeos_update_engine.InstallOperation_REPLACE),
			testPartition("vendor", vendorData, uint64(len(bootData)), chromeos_update_engine.InstallOperation_REPLACE),
		},
	}
	payloadPath := writeTestPayload(t, manifest, nil, append(bootData, vendorData...))

	p := NewPayload(payloadPath)
	if err := p.Open(); err != nil {
		t.Fatalf("open payload: %v", err)
	}
	defer p.file.Close()

	if err := p.Init(); err != nil {
		t.Fatalf("init payload: %v", err)
	}
	if err := p.SetConcurrency(2); err != nil {
		t.Fatalf("set concurrency: %v", err)
	}

	outDir := t.TempDir()
	if err := p.ExtractSelected(outDir, []string{"vendor"}); err != nil {
		t.Fatalf("extract selected: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "boot.img")); !os.IsNotExist(err) {
		t.Fatal("boot.img should not be extracted")
	}
	vendor, err := os.ReadFile(filepath.Join(outDir, "vendor.img"))
	if err != nil {
		t.Fatalf("read vendor image: %v", err)
	}
	if string(vendor) != string(vendorData) {
		t.Fatal("vendor image content mismatch")
	}
}

func TestExtractReturnsChecksumMismatch(t *testing.T) {
	block := bytesOf('C', blockSize)
	partition := testPartition("system", block, 0, chromeos_update_engine.InstallOperation_REPLACE)
	partition.Operations[0].DataSha256Hash = bytesOf(0x1, sha256.Size)

	manifest := &chromeos_update_engine.DeltaArchiveManifest{
		Partitions: []*chromeos_update_engine.PartitionUpdate{partition},
	}
	payloadPath := writeTestPayload(t, manifest, nil, block)

	p := NewPayload(payloadPath)
	if err := p.Open(); err != nil {
		t.Fatalf("open payload: %v", err)
	}
	defer p.file.Close()

	if err := p.Init(); err != nil {
		t.Fatalf("init payload: %v", err)
	}
	if err := p.SetConcurrency(1); err != nil {
		t.Fatalf("set concurrency: %v", err)
	}

	outDir := t.TempDir()
	err := p.ExtractAll(outDir)
	if err == nil {
		t.Fatal("expected checksum error")
	}
	if !strings.Contains(err.Error(), "Checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPayloadInitInvalidMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad_payload.bin")
	if err := os.WriteFile(path, []byte("BAD!"), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	p := NewPayload(path)
	if err := p.Open(); err != nil {
		t.Fatalf("open payload: %v", err)
	}
	defer p.file.Close()

	if err := p.Init(); err == nil {
		t.Fatal("expected init error for invalid magic")
	}
}

func testManifestSinglePartition(name string, data []byte, opType chromeos_update_engine.InstallOperation_Type) *chromeos_update_engine.DeltaArchiveManifest {
	return &chromeos_update_engine.DeltaArchiveManifest{
		Partitions: []*chromeos_update_engine.PartitionUpdate{
			testPartition(name, data, 0, opType),
		},
	}
}

func testPartition(name string, data []byte, dataOffset uint64, opType chromeos_update_engine.InstallOperation_Type) *chromeos_update_engine.PartitionUpdate {
	dataLength := uint64(len(data))
	size := uint64(len(data))
	hash := sha256.Sum256(data)
	start := uint64(0)
	numBlocks := uint64(len(data) / blockSize)

	return &chromeos_update_engine.PartitionUpdate{
		PartitionName: ptrString(name),
		NewPartitionInfo: &chromeos_update_engine.PartitionInfo{
			Size: &size,
		},
		Operations: []*chromeos_update_engine.InstallOperation{
			{
				Type:           &opType,
				DataOffset:     &dataOffset,
				DataLength:     &dataLength,
				DstExtents:     []*chromeos_update_engine.Extent{{StartBlock: &start, NumBlocks: &numBlocks}},
				DataSha256Hash: hash[:],
			},
		},
	}
}

func writeTestPayload(t *testing.T, manifest *chromeos_update_engine.DeltaArchiveManifest, sig *chromeos_update_engine.Signatures, data []byte) string {
	t.Helper()
	if sig == nil {
		sig = &chromeos_update_engine.Signatures{}
	}

	manifestBytes, err := proto.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	signatureBytes, err := proto.Marshal(sig)
	if err != nil {
		t.Fatalf("marshal signature: %v", err)
	}

	path := filepath.Join(t.TempDir(), "payload.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create payload: %v", err)
	}
	defer f.Close()

	if _, err := f.Write([]byte("CrAU")); err != nil {
		t.Fatalf("write magic: %v", err)
	}
	if err := binary.Write(f, binary.BigEndian, uint64(2)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := binary.Write(f, binary.BigEndian, uint64(len(manifestBytes))); err != nil {
		t.Fatalf("write manifest len: %v", err)
	}
	if err := binary.Write(f, binary.BigEndian, uint32(len(signatureBytes))); err != nil {
		t.Fatalf("write signature len: %v", err)
	}
	if _, err := f.Write(manifestBytes); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := f.Write(signatureBytes); err != nil {
		t.Fatalf("write signature: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write data: %v", err)
	}

	return path
}

func ptrString(v string) *string {
	return &v
}

func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}

	return out
}
