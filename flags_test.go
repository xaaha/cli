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

func (n *notBool) String() string { return string(*n) }

func (n *notBool) Set(s string) error { *n = notBool(s); return nil }

func (n *notBool) IsBoolFlag() bool { return false }

func flagsFirstTree(got *[]string, on *string, out io.Writer) *Command {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	fs.StringVar(on, "on", "", "Element id")
	return &Command{Name: "kapi", Config: &Config{Out: out}, SubCommands: []*Command{{
		Name: "say", Flags: fs, FlagsFirst: true,
		Run: func(args []string) error { *got = args; return nil },
	}}}
}

func TestExecuteParsesSubcommandFlags(t *testing.T) {
	var envVal string
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.StringVar(&envVal, "env", "", "environment")

	root := &Command{
		Name: "root",
		SubCommands: []*Command{
			{
				Name:  "init",
				Flags: fs,
				Run: func(_ []string) error {
					return nil
				},
			},
		},
	}

	if err := root.Execute([]string{"init", "-env", "staging"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envVal != "staging" {
		t.Errorf("expected env=staging, got %q", envVal)
	}
}

func TestFlagsAreReadAfterPositionalsByDefault(t *testing.T) {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	on := fs.String("on", "", "Element id")
	var got []string
	root := &Command{Name: "kapi", SubCommands: []*Command{{Name: "say", Flags: fs, Run: func(a []string) error { got = a; return nil }}}}
	if err := root.Execute([]string{"say", "page.html", "--on", "x"}); err != nil {
		t.Fatal(err)
	}
	if *on != "x" || !slices.Equal(got, []string{"page.html"}) {
		t.Errorf("on=%q args=%v", *on, got)
	}
}

func TestDoubleDashEndsFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "Verbose")
	var got []string
	root := &Command{Name: "app", Config: &Config{Out: io.Discard},
		SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func(args []string) error { got = args; return nil }}}}
	if err := root.Execute([]string{"leaf", "-v", "--", "a", "-x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !*verbose || !slices.Equal(got, []string{"a", "-x", "--help"}) {
		t.Errorf("verbose=%v args=%v", *verbose, got)
	}
}

func TestDoubleDashAsAFlagValueIsNotTheTerminator(t *testing.T) {
	cases := []struct {
		args     []string
		wantEnv  string
		wantVerb bool
		wantArgs []string
	}{
		{[]string{"leaf", "--env", "--", "foo", "--verbose"}, "--", true, []string{"foo"}},
		{[]string{"leaf", "a", "--", "-x", "-y"}, "", false, []string{"a", "-x", "-y"}},
		{[]string{"leaf", "--env=--", "a", "--", "--verbose"}, "--", false, []string{"a", "--verbose"}},
		{[]string{"leaf", "--verbose", "--", "a", "-x"}, "", true, []string{"a", "-x"}},
		{[]string{"leaf", "-", "x", "--", "-verbose"}, "", false, []string{"-", "x", "-verbose"}},
		{[]string{"leaf", "foo", "--verbose", "--", "x"}, "", true, []string{"foo", "x"}},
	}
	for _, tc := range cases {
		fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
		env := fs.String("env", "", "Env")
		verbose := fs.Bool("verbose", false, "Verbose")
		var got []string
		root := &Command{Name: "app", Config: &Config{Out: io.Discard},
			SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func(args []string) error { got = args; return nil }}}}
		if err := root.Execute(tc.args); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if *env != tc.wantEnv || *verbose != tc.wantVerb || !slices.Equal(got, tc.wantArgs) {
			t.Errorf("%v: env=%q verbose=%v args=%v", tc.args, *env, *verbose, got)
		}
	}
}

func TestDoubleDashAfterCustomBoolFlags(t *testing.T) {
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

func TestFlagsFirstStopsAtTheFirstPositional(t *testing.T) {
	cases := []struct {
		args   []string
		wantOn string
		want   []string
	}{
		{[]string{"say", "--on", "x", "page.html", "-", "item", "-5", "--on", "y"}, "x", []string{"page.html", "-", "item", "-5", "--on", "y"}},
		{[]string{"say", "page.html", "--help"}, "", []string{"page.html", "--help"}},
		{[]string{"say", "--", "-page.html"}, "", []string{"-page.html"}},
	}
	for _, tc := range cases {
		var got []string
		var on string
		if err := flagsFirstTree(&got, &on, io.Discard).Execute(tc.args); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if on != tc.wantOn || !slices.Equal(got, tc.want) {
			t.Errorf("%v: on=%q args=%v, want on=%q args=%v", tc.args, on, got, tc.wantOn, tc.want)
		}
	}
}

func TestFlagsFirstStillShowsHelpAndRejectsBadFlags(t *testing.T) {
	var got []string
	var on string
	var out bytes.Buffer
	if err := flagsFirstTree(&got, &on, &out).Execute([]string{"say", "--on", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if got != nil || !strings.Contains(out.String(), "LEARN MORE") {
		t.Errorf("want help before any positional, ran with %v", got)
	}
	err := flagsFirstTree(&got, &on, io.Discard).Execute([]string{"say", "--bogus", "page.html"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -bogus") {
		t.Errorf("error = %v", err)
	}
}

func TestCustomBoolFlagIsBoolEverywhere(t *testing.T) {
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

func TestIsBoolFlagFalseKeepsTheValueSlot(t *testing.T) {
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

func TestLongFlagWithoutItsShortPartnerIsShown(t *testing.T) {
	var out bytes.Buffer
	root := oneFlagTree(&Config{Out: &out, FlagAliases: map[string]string{"output": "o"}}, "output", "Output file")
	if err := root.Execute([]string{"run", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "  -output string") {
		t.Errorf("--output missing from help:\n%s", out.String())
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'--output[Output file]:value:'") {
		t.Errorf("--output missing from zsh:\n%s", zsh)
	}
}

func TestAppsOwnShortHAndVFlagsStayVisible(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.String("h", "", "Host to connect to")
	fs.Bool("v", false, "Verbose output")
	var out bytes.Buffer
	root := &Command{
		Name:        "app",
		Config:      &Config{Out: &out, HiddenFlags: map[string]bool{"debug": true}},
		SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}},
	}
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	for _, want := range []string{"  -h string", "  -v\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q:\n%s", want, out.String())
		}
	}
	if !strings.Contains(zsh, "'-h[Host to connect to]:value:'") || !strings.Contains(zsh, "'-v[Verbose output]'") {
		t.Errorf("zsh lost the app's -h or -v:\n%s", zsh)
	}
	if !strings.Contains(bash, "    -h) return 0 ;;") {
		t.Errorf("bash does not know -h takes a value:\n%s", bash)
	}
}

func TestFlagErrorNamesFullCommandPath(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"watch", "--bad"}, "See 'kapi watch --help' for usage"},
		{[]string{"w", "--bad"}, "See 'kapi watch --help' for usage"},
		{[]string{"page", "pin", "--bad"}, "See 'kapi page pin --help' for usage"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			err := kapiTree().Execute(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
