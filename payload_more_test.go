package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vbauerster/mpb/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestExtractReplaceXzInvalidData(t *testing.T) {
	badCompressed := []byte("not-a-valid-xz-stream")
	hash := sha256.Sum256(badCompressed)

	p := newExtractPayload(t, badCompressed)
	defer p.file.Close()

	out := createTempImage(t)
	defer out.Close()

	part := extractPartitionWithOperation(
		chromeos_update_engine.InstallOperation_REPLACE_XZ,
		badCompressed,
		hash[:],
		oneBlockExtent(),
	)
	err := p.Extract(part, out)
	if err == nil {
		t.Fatal("expected REPLACE_XZ error for invalid compressed data")
	}
}

func TestExtractReplaceBzInvalidData(t *testing.T) {
	badCompressed := []byte("not-a-valid-bz-stream")
	hash := sha256.Sum256(badCompressed)

	p := newExtractPayload(t, badCompressed)
	defer p.file.Close()

	out := createTempImage(t)
	defer out.Close()

	part := extractPartitionWithOperation(
		chromeos_update_engine.InstallOperation_REPLACE_BZ,
		badCompressed,
		hash[:],
		oneBlockExtent(),
	)
	err := p.Extract(part, out)
	if err == nil {
		t.Fatal("expected REPLACE_BZ error for invalid compressed data")
	}
}

func TestReadDataBlobOutOfRange(t *testing.T) {
	p := newExtractPayload(t, bytesOf('D', blockSize))
	defer p.file.Close()

	_, err := p.readDataBlob(int64(blockSize*2), int64(blockSize))
	if err == nil {
		t.Fatal("expected out-of-range readDataBlob error")
	}
}

func TestPayloadInitInvalidMetadataSignature(t *testing.T) {
	path := writeTestPayloadWithRawSignature(t, []byte{0xFF})

	p := NewPayload(path)
	if err := p.Open(); err != nil {
		t.Fatalf("open payload: %v", err)
	}
	defer p.file.Close()

	err := p.Init()
	if err == nil {
		t.Fatal("expected invalid metadata signature error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "proto") && !strings.Contains(strings.ToLower(err.Error()), "unmarshal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func BenchmarkCopyToExtentsSingleBlock(b *testing.B) {
	srcData := bytesOf('X', blockSize)
	extents := oneBlockExtent()
	outPath := filepath.Join(b.TempDir(), "single.img")
	out, err := os.Create(outPath)
	if err != nil {
		b.Fatalf("create output file: %v", err)
	}
	defer out.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Truncate(0); err != nil {
			b.Fatalf("truncate output: %v", err)
		}
		if _, err := out.Seek(0, 0); err != nil {
			b.Fatalf("seek output: %v", err)
		}
		if _, err := copyToExtents(out, extents, bytes.NewReader(srcData)); err != nil {
			b.Fatalf("copyToExtents failed: %v", err)
		}
	}
}

func BenchmarkExtractReplaceSingleBlock(b *testing.B) {
	block := bytesOf('R', blockSize)
	hash := sha256.Sum256(block)
	part := extractPartitionWithOperation(chromeos_update_engine.InstallOperation_REPLACE, block, hash[:], oneBlockExtent())

	p := newBenchPayload(b, block)
	defer p.file.Close()
	out := newBenchOutputFile(b)
	defer out.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Truncate(0); err != nil {
			b.Fatalf("truncate output: %v", err)
		}
		if _, err := out.Seek(0, 0); err != nil {
			b.Fatalf("seek output: %v", err)
		}
		if err := p.Extract(part, out); err != nil {
			b.Fatalf("extract replace failed: %v", err)
		}
	}
}

func BenchmarkExtractZeroSingleBlock(b *testing.B) {
	part := extractPartitionWithOperation(chromeos_update_engine.InstallOperation_ZERO, nil, nil, oneBlockExtent())

	p := newBenchPayload(b, nil)
	defer p.file.Close()
	out := newBenchOutputFile(b)
	defer out.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Truncate(0); err != nil {
			b.Fatalf("truncate output: %v", err)
		}
		if _, err := out.Seek(0, 0); err != nil {
			b.Fatalf("seek output: %v", err)
		}
		if err := p.Extract(part, out); err != nil {
			b.Fatalf("extract zero failed: %v", err)
		}
	}
}

func newBenchPayload(b *testing.B, blob []byte) *Payload {
	b.Helper()
	path := filepath.Join(b.TempDir(), "bench_blob.bin")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		b.Fatalf("write blob: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		b.Fatalf("open blob: %v", err)
	}

	return &Payload{
		file:       f,
		dataOffset: 0,
		progress:   mpb.New(mpb.WithOutput(io.Discard)),
	}
}

func newBenchOutputFile(b *testing.B) *os.File {
	b.Helper()
	outPath := filepath.Join(b.TempDir(), "bench.img")
	out, err := os.Create(outPath)
	if err != nil {
		b.Fatalf("create output: %v", err)
	}

	return out
}

func writeTestPayloadWithRawSignature(t *testing.T, rawSig []byte) string {
	t.Helper()
	block := bytesOf('S', blockSize)
	manifest := testManifestSinglePartition("boot", block, chromeos_update_engine.InstallOperation_REPLACE)
	manifestBytes, err := protoMarshalManifest(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	path := filepath.Join(t.TempDir(), "payload_invalid_sig.bin")
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
	if err := binary.Write(f, binary.BigEndian, uint32(len(rawSig))); err != nil {
		t.Fatalf("write signature len: %v", err)
	}
	if _, err := f.Write(manifestBytes); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := f.Write(rawSig); err != nil {
		t.Fatalf("write raw signature: %v", err)
	}
	if _, err := f.Write(block); err != nil {
		t.Fatalf("write block: %v", err)
	}

	return path
}

func protoMarshalManifest(m *chromeos_update_engine.DeltaArchiveManifest) ([]byte, error) {
	return proto.Marshal(m)
}
