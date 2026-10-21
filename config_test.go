package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func treeWithAlias(name, long, short string, out *bytes.Buffer) *Command {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.String(long, "", "the "+long+" flag")
	fs.String(short, "", "the "+long+" flag")
	return &Command{
		Name:   name,
		Flags:  fs,
		Run:    func([]string) error { return nil },
		Config: &Config{FlagAliases: map[string]string{long: short}, Out: out},
	}
}

func TestD5ConfigIsPerTree(t *testing.T) {
	cases := []struct{ name, long, short, other string }{
		{"one", "extra", "x", "-y, --yonder"},
		{"two", "yonder", "y", "-x, --extra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for range 50 {
				var out bytes.Buffer
				if err := treeWithAlias(tc.name, tc.long, tc.short, &out).Execute([]string{"--help"}); err != nil {
					t.Fatal(err)
				}
				want := "-" + tc.short + ", --" + tc.long
				if !strings.Contains(out.String(), want) {
					t.Fatalf("help missing %q:\n%s", want, out.String())
				}
				if strings.Contains(out.String(), tc.other) {
					t.Fatalf("help shows the other tree's alias %q:\n%s", tc.other, out.String())
				}
			}
		})
	}
}

func TestD5HelpGoesToConfiguredWriter(t *testing.T) {
	var out bytes.Buffer
	root := &Command{Name: "kapi", Long: "Shared pages", Config: &Config{Out: &out}}
	stdout := captureStdout(t, func() {
		if err := root.Execute([]string{"--help"}); err != nil {
			t.Fatal(err)
		}
	})
	if stdout != "" {
		t.Errorf("help leaked to stdout: %q", stdout)
	}
	if !strings.Contains(out.String(), "Shared pages") {
		t.Errorf("configured writer missing help: %q", out.String())
	}
}

func TestD5DefaultsWithoutConfig(t *testing.T) {
	fs := flag.NewFlagSet("kapi", flag.ContinueOnError)
	fs.Bool("version", false, "Print the version")
	fs.Bool("v", false, "Print the version")
	fs.String("file-path", "", "A path")
	fs.String("fp", "", "A path")
	root := &Command{Name: "kapi", Flags: fs, Run: func([]string) error { return nil }}
	out := captureStdout(t, root.PrintHelp)
	if strings.Contains(out, "-v") {
		t.Errorf("-v should be hidden by default:\n%s", out)
	}
	if strings.Contains(out, "-fp, --file-path") {
		t.Errorf("hulak's fp alias must not be a default:\n%s", out)
	}
}
