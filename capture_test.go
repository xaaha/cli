package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns everything
// fn wrote. Stdout is restored even if fn panics. Used by snapshot and help-
// rendering tests that assert on printed output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}
