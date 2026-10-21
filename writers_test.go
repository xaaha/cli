package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewFPartialConfigKeepsDefaults(t *testing.T) {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	for _, name := range []string{"help", "h", "version", "v"} {
		fs.Bool(name, false, "builtin")
	}
	var out bytes.Buffer
	root := &Command{Name: "app", Flags: fs, Run: func([]string) error { return nil }, Config: &Config{Out: &out}}
	root.PrintHelp()
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "  -h") || strings.HasPrefix(line, "  -v") || strings.HasPrefix(line, "  -help") {
			t.Errorf("default hidden flag shown with a partial Config: %q\n%s", line, out.String())
		}
	}
}

func TestReviewFEveryHelpSectionUsesOut(t *testing.T) {
	cases := map[string][]string{
		"help_root":     {"--help"},
		"help_run":      {"run", "--help"},
		"help_secrets":  {"env", "--help"},
		"help_keys_set": {"secrets", "keys", "set", "-h"},
	}
	for golden, args := range cases {
		t.Run(golden, func(t *testing.T) {
			var out bytes.Buffer
			tree := fixtureTree()
			tree.Config.Out = &out
			stdout := captureStdout(t, func() {
				if err := tree.Execute(args); err != nil {
					t.Fatal(err)
				}
			})
			if stdout != "" {
				t.Errorf("help leaked to stdout: %q", stdout)
			}
			want, err := os.ReadFile(filepath.Join("testdata", golden+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) {
				t.Errorf("Config.Out differs from %s.golden\n%s", golden, out.String())
			}
		})
	}
}

func TestReviewFUnknownCommandWritesErr(t *testing.T) {
	var out, errw bytes.Buffer
	tree := fixtureTree()
	tree.Config.Out, tree.Config.Err = &out, &errw
	if err := tree.Execute([]string{"bogus"}); err == nil {
		t.Fatal("want an error for an unknown command")
	}
	if errw.String() != "\n" {
		t.Errorf("Config.Err = %q, want the blank separator line", errw.String())
	}
}
