package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestRunNoArgsReturnsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(nil, &stdout, &stderr)
	if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", stderr.String())
	}
}

func TestRunMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"does-not-exist.bin"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected file-not-exist error")
	}
	if !strings.Contains(err.Error(), "file does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractPayloadBinFromZip(t *testing.T) {
	payloadPath := writeTestPayload(t, testManifestSinglePartition("boot", bytesOf('A', blockSize), chromeos_update_engine.InstallOperation_REPLACE), nil, bytesOf('A', blockSize))
	zipPath := createZipWithPayload(t, payloadPath)

	extractedPath, err := extractPayloadBin(zipPath)
	if err != nil {
		t.Fatalf("extractPayloadBin failed: %v", err)
	}
	defer os.Remove(extractedPath)

	got, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("read extracted payload: %v", err)
	}
	src, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("read source payload: %v", err)
	}
	if !bytes.Equal(got, src) {
		t.Fatal("zip extracted payload mismatch")
	}
}

func TestRunListModeWithZipInput(t *testing.T) {
	payloadPath := writeTestPayload(t, testManifestSinglePartition("boot", bytesOf('A', blockSize), chromeos_update_engine.InstallOperation_REPLACE), nil, bytesOf('A', blockSize))
	zipPath := createZipWithPayload(t, payloadPath)

	var stdout, stderr bytes.Buffer
	err := run([]string{"-l", zipPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run list with zip failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Please wait while extracting payload.bin from the archive.") {
		t.Fatalf("expected zip extraction message, got %q", stdout.String())
	}
}

func TestRunExtractWithOutputDir(t *testing.T) {
	block := bytesOf('Q', blockSize)
	payloadPath := writeTestPayload(t, testManifestSinglePartition("boot", block, chromeos_update_engine.InstallOperation_REPLACE), nil, block)

	outDir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	err := run([]string{"-o", outDir, "-c", "1", payloadPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run extract failed: %v", err)
	}

	imgPath := filepath.Join(outDir, "boot.img")
	got, err := os.ReadFile(imgPath)
	if err != nil {
		t.Fatalf("read extracted image: %v", err)
	}
	if string(got) != string(block) {
		t.Fatal("extracted content mismatch")
	}
}

func TestRunInvalidConcurrency(t *testing.T) {
	block := bytesOf('A', blockSize)
	payloadPath := writeTestPayload(t, testManifestSinglePartition("boot", block, chromeos_update_engine.InstallOperation_REPLACE), nil, block)

	var stdout, stderr bytes.Buffer
	err := run([]string{"-c", "0", payloadPath}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "invalid concurrency") {
		t.Fatalf("expected invalid concurrency error, got %v", err)
	}
}

func TestRunInvalidZipInput(t *testing.T) {
	badZip := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(badZip, []byte("not-a-zip"), 0o644); err != nil {
		t.Fatalf("write bad zip: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"-l", badZip}, &stdout, &stderr)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not a valid zip archive") {
		t.Fatalf("expected invalid zip error, got %v", err)
	}
}

func createZipWithPayload(t *testing.T, payloadPath string) string {
	t.Helper()
	zipPath := filepath.Join(t.TempDir(), "payload.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	w, err := zw.Create("payload.bin")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}

	payloadBytes, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("read payload file: %v", err)
	}
	if _, err := w.Write(payloadBytes); err != nil {
		t.Fatalf("write zip payload: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return zipPath
}
