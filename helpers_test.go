package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func tryScripts(root *Command) (zsh, bash string, err error) {
	var z, b bytes.Buffer
	if err := WriteZshCompletion(&z, root); err != nil {
		return "", "", err
	}
	if err := WriteBashCompletion(&b, root); err != nil {
		return "", "", err
	}
	return z.String(), b.String(), nil
}

func scripts(t *testing.T, root *Command) (zsh, bash string) {
	t.Helper()
	var z, b bytes.Buffer
	if err := WriteZshCompletion(&z, root); err != nil {
		t.Fatal(err)
	}
	if err := WriteBashCompletion(&b, root); err != nil {
		t.Fatal(err)
	}
	return z.String(), b.String()
}

func shellPath(t *testing.T, shell string) string {
	t.Helper()
	bin, err := exec.LookPath(shell)
	if err == nil {
		return bin
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("%s is required on CI for the syntax check", shell)
	}
	t.Skipf("%s not installed", shell)
	return ""
}

func checkSyntax(t *testing.T, shell, src string) {
	t.Helper()
	bin := shellPath(t, shell)
	path := filepath.Join(t.TempDir(), "script."+shell)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "-n", path).CombinedOutput(); err != nil {
		t.Errorf("%s -n: %v\n%s\n%s", shell, err, out, src)
	}
}

type counter int

func (c *counter) String() string { return strconv.Itoa(int(*c)) }

func (c *counter) Set(string) error { *c++; return nil }

func (c *counter) IsBoolFlag() bool { return true }
