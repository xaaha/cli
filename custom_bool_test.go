package cli

import (
	"bytes"
	"flag"
	"strconv"
	"strings"
	"testing"
)

type counter int

func (c *counter) String() string   { return strconv.Itoa(int(*c)) }
func (c *counter) Set(string) error { *c++; return nil }
func (c *counter) IsBoolFlag() bool { return true }

func TestReviewAPCustomBoolFlagIsBoolEverywhere(t *testing.T) {
	var n counter
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	fs.Var(&n, "verbose", "More output")
	fs.String("on", "true", "A string whose default looks like a bool")
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{{Name: "deploy", Flags: fs, Run: func([]string) error { return nil }}}}
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	help := out.String()
	if strings.Contains(help, "-verbose string") || !strings.Contains(help, "-on string") {
		t.Errorf("help types are wrong:\n%s", help)
	}
	if !strings.Contains(zsh, "'--verbose[More output]'") || !strings.Contains(zsh, "'--on[A string whose default looks like a bool]:value:'") {
		t.Errorf("zsh value slots are wrong:\n%s", zsh)
	}
	if strings.Contains(bash, "--verbose|") || !strings.Contains(bash, "--on|-on) return 0") {
		t.Errorf("bash value flags are wrong:\n%s", bash)
	}
}
