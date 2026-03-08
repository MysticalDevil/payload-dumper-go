package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReaderReadFromOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r := NewReader(path, 3)
	defer r.Close()

	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 4 {
		t.Fatalf("unexpected bytes read: got %d, want 4", n)
	}
	if got := string(buf); got != "3456" {
		t.Fatalf("unexpected read content: %q", got)
	}
}

func TestReaderCloseWithoutOpen(t *testing.T) {
	r := NewReader("non-existent", 0)
	if err := r.Close(); err != nil {
		t.Fatalf("close should be no-op: %v", err)
	}
}
