package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata")

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run go test -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from %s\n--- got ---\n%s\n--- want ---\n%s", name, path, got, want)
	}
}

func executeOutput(t *testing.T, args ...string) string {
	t.Helper()
	var err error
	out := captureStdout(t, func() { err = fixtureTree().Execute(args) })
	if err != nil {
		out += "error: " + err.Error() + "\n"
	}
	return out
}

func TestGoldenOutput(t *testing.T) {
	cases := []struct {
		name string
		got  func(t *testing.T) string
	}{
		{"help_root", func(t *testing.T) string { return executeOutput(t, "--help") }},
		{"help_run", func(t *testing.T) string { return executeOutput(t, "run", "--help") }},
		{"help_secrets", func(t *testing.T) string { return executeOutput(t, "env", "--help") }},
		{"help_keys_set", func(t *testing.T) string { return executeOutput(t, "secrets", "keys", "set", "-h") }},
		{"error_unknown_command", func(t *testing.T) string { return executeOutput(t, "bogus") }},
		{"error_bad_flag", func(t *testing.T) string { return executeOutput(t, "run", "--nope") }},
		{"error_bad_flag_nested", func(t *testing.T) string { return executeOutput(t, "secrets", "keys", "set", "--nope") }},
		{"help_hello", func(t *testing.T) string {
			var b bytes.Buffer
			hello := &Command{Name: "hello", Short: "Say hello", Config: &Config{Out: &b}, Run: func([]string) error { return nil }}
			if err := hello.Execute([]string{"--help"}); err != nil {
				t.Fatal(err)
			}
			return b.String()
		}},
		{"completion_zsh", func(t *testing.T) string {
			var b bytes.Buffer
			if err := WriteZshCompletion(&b, fixtureTree()); err != nil {
				t.Fatal(err)
			}
			return b.String()
		}},
		{"completion_bash", func(t *testing.T) string {
			var b bytes.Buffer
			if err := WriteBashCompletion(&b, fixtureTree()); err != nil {
				t.Fatal(err)
			}
			return b.String()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { checkGolden(t, tc.name, tc.got(t)) })
	}
}
