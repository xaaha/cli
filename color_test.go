package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func helpWith(t *testing.T, cfg *Config) string {
	t.Helper()
	root := &Command{Name: "kapi", Long: "Shared pages", SubCommands: []*Command{{Name: "watch", Short: "Stream events"}}, Config: cfg}
	root.PrintHelp()
	switch w := cfg.Out.(type) {
	case *bytes.Buffer:
		return w.String()
	case *os.File:
		b, err := os.ReadFile(w.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	t.Fatalf("unexpected writer %T", cfg.Out)
	return ""
}

func TestD13ColorOnlyOnTerminal(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "help.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	cases := []struct {
		name      string
		cfg       *Config
		wantColor bool
	}{
		{"auto to a buffer", &Config{Out: &bytes.Buffer{}}, false},
		{"auto to a regular file", &Config{Out: file}, false},
		{"forced on", &Config{Out: &bytes.Buffer{}, Color: ColorAlways}, true},
		{"forced off", &Config{Out: &bytes.Buffer{}, Color: ColorNever}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := helpWith(t, tc.cfg)
			if got := strings.Contains(out, "\x1b["); got != tc.wantColor {
				t.Errorf("colour = %v, want %v:\n%q", got, tc.wantColor, out)
			}
		})
	}
}

func TestD13NoColorEnvBeatsTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorEnabled(ColorAuto, func() bool { return true }) {
		t.Error("NO_COLOR must disable colour on a terminal")
	}
	t.Setenv("NO_COLOR", "")
	if !colorEnabled(ColorAuto, func() bool { return true }) {
		t.Error("a terminal without NO_COLOR should get colour")
	}
	if !colorEnabled(ColorAlways, func() bool { return false }) {
		t.Error("ColorAlways must colour non-terminals")
	}
}
