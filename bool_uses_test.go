package cli

import (
	"bytes"
	"flag"
	"io"
	"slices"
	"strings"
	"testing"
)

type notBool string

func (n *notBool) String() string     { return string(*n) }
func (n *notBool) Set(s string) error { *n = notBool(s); return nil }
func (n *notBool) IsBoolFlag() bool   { return false }

func TestReviewAVBoolFlagsDoNotSwallowTheSubcommand(t *testing.T) {
	for _, arg := range []string{"--verbose", "-c"} {
		var c counter
		fs := flag.NewFlagSet("app", flag.ContinueOnError)
		fs.Bool("verbose", false, "Verbose")
		fs.Var(&c, "c", "Count")
		ran := false
		root := &Command{Name: "app", Flags: fs, Config: &Config{Out: io.Discard},
			SubCommands: []*Command{{Name: "status", Run: func([]string) error { ran = true; return nil }}}}
		if err := root.Execute([]string{arg, "status"}); err != nil {
			t.Fatalf("%s: %v", arg, err)
		}
		if !ran {
			t.Errorf("%s swallowed the subcommand name", arg)
		}
	}
}

func TestReviewAVDoubleDashAfterCustomFlags(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"leaf", "-v", "--", "a", "-x"}, []string{"a", "-x"}},
		{[]string{"leaf", "--on", "--", "a", "--verbose"}, []string{"a"}},
	}
	for _, tc := range cases {
		var c counter
		fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
		fs.Var(&c, "v", "Count")
		on := fs.String("on", "true", "Looks like a bool")
		verbose := fs.Bool("verbose", false, "Verbose")
		var got []string
		root := &Command{Name: "app", Config: &Config{Out: io.Discard},
			SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func(args []string) error { got = args; return nil }}}}
		if err := root.Execute(tc.args); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%v: args=%v on=%q verbose=%v", tc.args, got, *on, *verbose)
		}
	}
}

func TestReviewAVBoolFlagsAreNotCheckedForCompleters(t *testing.T) {
	var c counter
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Bool("quiet", false, "Quiet")
	fs.Var(&c, "c", "Count")
	root := &Command{Name: "app", Config: &Config{FlagCompleters: map[string]string{"quiet": "nope", "c": "nope"}},
		SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}}}
	if _, _, err := tryScripts(root); err != nil {
		t.Errorf("bool flags take no value, so their completer is never used: %v", err)
	}
}

func TestReviewAVValueWithIsBoolFlagFalseKeepsItsValueSlot(t *testing.T) {
	var n notBool
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Var(&n, "mode", "Mode")
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}}}
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	if !strings.Contains(out.String(), "-mode string") || !strings.Contains(zsh, "'--mode[Mode]:value:'") || !strings.Contains(bash, "--mode|-mode) return 0") {
		t.Errorf("value slot lost:\nhelp:\n%s\nzsh:\n%s\nbash:\n%s", out.String(), zsh, bash)
	}
}
