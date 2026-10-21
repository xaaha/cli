package cli

import (
	"bytes"
	"flag"
	"io"
	"slices"
	"strings"
	"testing"
	"time"
)

type endpointValue string

func (e *endpointValue) String() string { return string(*e) }

func (e *endpointValue) Set(s string) error { *e = endpointValue(s); return nil }

func helpLines(help string) map[string]bool {
	lines := map[string]bool{}
	for _, l := range strings.Split(help, "\n") {
		if strings.HasPrefix(l, "  -") {
			lines[strings.Fields(l)[0]] = true
		}
	}
	return lines
}

func TestPrintHelpShowsEverySection(t *testing.T) {
	fs := flag.NewFlagSet("gql", flag.ContinueOnError)
	fs.String("env", "", "Environment to use")

	cmd := &Command{
		Name:  "gql",
		Short: "GraphQL explorer",
		Long:  "Open the GraphQL explorer for files and directories",
		Flags: fs,
		Args: []ArgDef{
			{Name: "path", Required: true, Desc: "File or directory path"},
		},
		SubCommands: []*Command{
			{Name: "create", Short: "Create a new GraphQL file"},
		},
	}

	output := captureStdout(t, func() {
		cmd.PrintHelp()
	})

	checks := []string{
		"Open the GraphQL explorer",
		"COMMANDS",
		"create",
		"Create a new GraphQL file",
		"FLAGS",
		"-env",
		"ARGUMENTS",
		"<path>",
		"(required)",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("help output missing %q\nGot:\n%s", check, output)
		}
	}
}

func TestPrintHelpShowsAliases(t *testing.T) {
	cmd := &Command{
		Name: "root",
		Long: "Root command",
		SubCommands: []*Command{
			{Name: "list", Aliases: []string{"ls"}, Short: "List items"},
			{Name: "delete", Aliases: []string{"rm", "remove"}, Short: "Delete an item"},
			{Name: "get", Short: "Get an item"},
		},
	}

	output := captureStdout(t, func() {
		cmd.PrintHelp()
	})

	checks := []string{
		"list (ls)",
		"delete (rm, remove)",
		"get",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("help output missing %q\nGot:\n%s", check, output)
		}
	}

	// "get" should NOT have parentheses since it has no aliases
	if strings.Contains(output, "get (") {
		t.Errorf("get should not show alias parentheses\nGot:\n%s", output)
	}
}

func TestPrintHelpIncludesExamples(t *testing.T) {
	cmd := &Command{
		Name: "gql",
		Long: "GraphQL explorer",
		Examples: []Example{
			{Command: "hulak gql .", Description: "All files in current dir"},
		},
	}

	output := captureStdout(t, func() {
		cmd.PrintHelp()
	})

	checks := []string{"EXAMPLES", "hulak gql .", "All files in current dir"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("help output missing %q\nGot:\n%s", check, output)
		}
	}
}

func TestPrintHelpFormatsExamples(t *testing.T) {
	cmd := &Command{
		Name:     "kapi",
		Long:     "Shared pages",
		Examples: []Example{{Command: "kapi watch page.html", Description: "Stream events"}},
	}
	out := captureStdout(t, cmd.PrintHelp)
	for _, want := range []string{"EXAMPLES", "  $ kapi watch page.html", "    Stream events"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q\n%s", want, out)
		}
	}
}

func TestHelpTypeHintsMatchTheFlagType(t *testing.T) {
	var ep endpointValue
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Var(&ep, "endpoint", "Endpoint")
	fs.Int("count", 0, "Count")
	fs.Int64("big", 0, "Big")
	fs.Uint("workers", 0, "Workers")
	fs.Uint64("bytes", 0, "Bytes")
	fs.Float64("ratio", 0, "Ratio")
	fs.Duration("wait", time.Second, "Wait")
	fs.String("name", "", "Name")
	var out bytes.Buffer
	(&Command{Name: "run", Flags: fs, Config: &Config{Out: &out}}).PrintHelp()
	for _, want := range []string{"-endpoint string", "-count int", "-big int", "-workers uint", "-bytes uint", "-ratio float", "-wait duration", "-name string"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestHelpNamesRootProgram(t *testing.T) {
	out := captureStdout(t, func() {
		if err := kapiTree().Execute([]string{"page", "--help"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Use `kapi <command> --help`") {
		t.Errorf("help does not name the root program:\n%s", out)
	}
	if strings.Contains(out, "hulak") {
		t.Errorf("help mentions hulak:\n%s", out)
	}
}

func TestLeafHelpShowsUsageAndDescription(t *testing.T) {
	var out bytes.Buffer
	hello := &Command{Name: "hello", Short: "Say hello", Config: &Config{Out: &out}, Run: func([]string) error { return nil }}
	if err := hello.Execute([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if want := "Say hello\n\nUSAGE\n  hello\n"; out.String() != want {
		t.Errorf("root leaf help = %q, want %q", out.String(), want)
	}

	out.Reset()
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.String("out", "", "Out path")
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{{
		Name: "run", Short: "Run it", Long: "Run one file.", Flags: fs,
		Args: []ArgDef{{Name: "path", Required: true}, {Name: "more"}},
		Run:  func([]string) error { return nil },
	}}}
	if err := root.Execute([]string{"run", "--help"}); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, want := range []string{"Run one file.\n\nUSAGE\n  app run [flags] <path> [<more>]\n\nFLAGS\n", "ARGUMENTS"} {
		if !strings.Contains(help, want) {
			t.Errorf("leaf help missing %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "Run it") || strings.Contains(help, "LEARN MORE") {
		t.Errorf("leaf help shows Short over Long or a LEARN MORE hint:\n%s", help)
	}

	out.Reset()
	root.PrintHelp()
	if strings.Contains(out.String(), "USAGE") || !strings.Contains(out.String(), "LEARN MORE") {
		t.Errorf("group help changed:\n%s", out.String())
	}
}

func TestPrintHelpAfterExecuteUsesTheRoot(t *testing.T) {
	var out bytes.Buffer
	root, _, leaf := linkTree(&out)
	if err := root.Execute([]string{"other"}); err != nil {
		t.Fatal(err)
	}
	leaf.PrintHelp()
	if !strings.Contains(out.String(), "USAGE\n  app grp leaf\n") {
		t.Errorf("leaf help did not use the root's name and writer: %q", out.String())
	}
}

func TestHelpArgumentsDoNotCallRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"--help flag", []string{"--help"}},
		{"-h flag", []string{"-h"}},
		{"help subcommand", []string{"help"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runCalled := false
			cmd := &Command{
				Name: "test",
				Long: "Test command help text",
				Run: func(_ []string) error {
					runCalled = true
					return nil
				},
			}

			if err := cmd.Execute(tc.args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if runCalled {
				t.Error("Run should not be called when help is requested")
			}
		})
	}
}

func TestSubcommandHelpDoesNotCallRun(t *testing.T) {
	runCalled := false
	root := &Command{
		Name: "root",
		SubCommands: []*Command{
			{
				Name:  "init",
				Short: "Initialize project",
				Long:  "Initialize a new hulak project with default settings",
				Run: func(_ []string) error {
					runCalled = true
					return nil
				},
			},
		},
	}

	// "init --help" should show init's help, not run init
	if err := root.Execute([]string{"init", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runCalled {
		t.Error("Run should not be called when help is requested for subcommand")
	}
}

func TestOnlyExactHelpArgumentsAreHelp(t *testing.T) {
	cases := []struct {
		arg      string
		short    string
		wantHelp bool
	}{
		{"--help", "h", true},
		{"-h", "h", true},
		{"help", "h", true},
		{"-", "", false},
		{"--", "", false},
		{"-", "h", false},
		{"---help", "h", false},
		{"--h", "h", false},
		{"-help", "h", false},
		{"--help=false", "h", false},
		{"--helpx", "h", false},
		{"HELP", "h", false},
	}
	for _, tc := range cases {
		t.Run(tc.arg+"/short="+tc.short, func(t *testing.T) {
			var got []string
			ran := false
			var out bytes.Buffer
			root := &Command{
				Name:   "app",
				Config: &Config{Out: &out, FlagAliases: map[string]string{"help": tc.short}},
				SubCommands: []*Command{
					{Name: "cat", Run: func(args []string) error { ran, got = true, args; return nil }},
				},
			}
			if err := root.Execute([]string{"cat", tc.arg}); err != nil {
				t.Fatal(err)
			}
			if tc.wantHelp && (ran || !strings.Contains(out.String(), "USAGE")) {
				t.Errorf("want help, ran=%v", ran)
			}
			if !tc.wantHelp && (!ran || !slices.Equal(got, []string{tc.arg})) {
				t.Errorf("want Run(%q), ran=%v args=%v help=%q", tc.arg, ran, got, out.String())
			}
		})
	}
}

func TestHelpWordWorksAtEveryLevel(t *testing.T) {
	leafFlags := flag.NewFlagSet("leaf", flag.ContinueOnError)
	leafFlags.String("out", "", "Out")
	grpFlags := flag.NewFlagSet("grp", flag.ContinueOnError)
	grpFlags.String("env", "", "Env")
	for _, args := range [][]string{{"help"}, {"grp", "help"}, {"grp", "leaf", "help"}, {"grp", "--env", "p", "--help"}} {
		ran := false
		var out bytes.Buffer
		root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
			{Name: "grp", Flags: grpFlags, SubCommands: []*Command{
				{Name: "leaf", Flags: leafFlags, Run: func([]string) error { ran = true; return nil }},
			}},
		}}
		if err := root.Execute(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if ran || !strings.Contains(out.String(), "USAGE") && !strings.Contains(out.String(), "LEARN MORE") {
			t.Errorf("%v: ran=%v, want help", args, ran)
		}
	}
}

func TestHelpAfterAPositionalStillShowsHelp(t *testing.T) {
	for _, args := range [][]string{{"leaf", "a", "--help", "--", "x"}, {"leaf", "--verbose", "--help", "foo", "--bad"}} {
		fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
		fs.Bool("verbose", false, "Verbose")
		ran := false
		var out bytes.Buffer
		root := &Command{Name: "app", Config: &Config{Out: &out},
			SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func([]string) error { ran = true; return nil }}}}
		if err := root.Execute(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if ran || !strings.Contains(out.String(), "USAGE") {
			t.Errorf("%v: ran=%v, want help", args, ran)
		}
	}
}

func TestHelpFalseRunsTheCommand(t *testing.T) {
	ran := false
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	root := &Command{Name: "app", Config: &Config{Out: &bytes.Buffer{}}, SubCommands: []*Command{
		{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
	}}
	if err := root.Execute([]string{"run", "--help=false"}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("--help=false should run the command")
	}
}

func TestAppsOwnHelpFlagWinsAsFirstArgument(t *testing.T) {
	for _, arg := range []string{"--help", "-help"} {
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		manual := fs.Bool("help", false, "Show the manual")
		ran := false
		root := &Command{Name: "app", Config: &Config{Out: &bytes.Buffer{}}, SubCommands: []*Command{
			{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
		}}
		if err := root.Execute([]string{"run", arg}); err != nil {
			t.Fatal(err)
		}
		if !ran || !*manual {
			t.Errorf("%s: ran=%v manual=%v, want the app's own flag set", arg, ran, *manual)
		}
	}
}

func TestAppsOwnShortHFlagWorksAsFirstArgument(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	host := fs.String("h", "", "Host")
	ran := false
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
		{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
	}}
	if err := root.Execute([]string{"run", "-h", "example.com"}); err != nil {
		t.Fatal(err)
	}
	if !ran || *host != "example.com" || out.Len() != 0 {
		t.Errorf("ran=%v host=%q help=%q", ran, *host, out.String())
	}
}

func TestAppsOwnLongHelpIsNotPairedWithInjectedShort(t *testing.T) {
	root, out := helpTree(&Config{}, func(fs *flag.FlagSet) { fs.Bool("help", false, "Show the manual") })
	if err := root.Execute([]string{"run", "--out", "x"}); err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	lines := helpLines(out.String())
	if !lines["-help"] || lines["-h"] || lines["-h,"] {
		t.Errorf("want the app's -help listed and no injected -h:\n%s", out.String())
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'--help[Show the manual]'") || strings.Contains(zsh, "-h[") {
		t.Errorf("zsh should offer only the app's --help:\n%s", zsh)
	}
}

func TestOnlyInjectedHelpFlagsAreHidden(t *testing.T) {
	root, out := helpTree(&Config{}, func(fs *flag.FlagSet) { fs.String("h", "", "Host") })
	if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	lines := helpLines(out.String())
	if !lines["-h"] || lines["-help"] {
		t.Errorf("want the app's -h listed and the injected --help hidden:\n%s", out.String())
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'-h[Host]:value:'") {
		t.Errorf("zsh lost the app's -h after Execute:\n%s", zsh)
	}
}

func TestHiddenFlagsFalseShowsInjectedHelp(t *testing.T) {
	root, out := helpTree(&Config{HiddenFlags: map[string]bool{"h": false}}, func(*flag.FlagSet) {})
	if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "-h, --help") {
		t.Errorf("explicit HiddenFlags h=false should list -h, --help:\n%s", out.String())
	}
}

func TestShownHelpFlagsDoNotDependOnArgumentOrder(t *testing.T) {
	for _, args := range [][]string{{"run", "--help"}, {"run", "--out", "x", "--help"}} {
		root, out := helpTree(&Config{HiddenFlags: map[string]bool{"h": false}}, func(*flag.FlagSet) {})
		if err := root.Execute(args); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "-h, --help") {
			t.Errorf("%v: -h, --help not listed:\n%s", args, out.String())
		}
	}
	root, _ := helpTree(&Config{HiddenFlags: map[string]bool{"h": false}}, func(*flag.FlagSet) {})
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "{--help,-h}") {
		t.Errorf("a fresh tree's zsh script does not offer the shown help flags:\n%s", zsh)
	}
}

func TestCustomShortHelpAliasRequestsHelp(t *testing.T) {
	root, out := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(*flag.FlagSet) {})
	if err := root.Execute([]string{"run", "--out", "x", "-H"}); err != nil {
		t.Fatalf("-H should request help: %v", err)
	}
	if !strings.Contains(out.String(), "FLAGS") {
		t.Fatalf("-H did not print help:\n%s", out.String())
	}
	if lines := helpLines(out.String()); lines["-h"] || lines["-help"] || lines["-H,"] {
		t.Errorf("injected help flags listed:\n%s", out.String())
	}
}

func TestCustomShortHelpAliasReplacesH(t *testing.T) {
	root, _ := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(*flag.FlagSet) {})
	if err := root.Execute([]string{"run", "--out", "x", "-h"}); err == nil || !strings.Contains(err.Error(), "-h") {
		t.Errorf("-h with help->H: error = %v, want an unknown flag", err)
	}

	ran := false
	var out bytes.Buffer
	bare := &Command{Name: "app", Config: &Config{Out: &out, FlagAliases: map[string]string{"help": "H"}}, SubCommands: []*Command{
		{Name: "run", Run: func([]string) error { ran = true; return nil }},
	}}
	if err := bare.Execute([]string{"run", "-H"}); err != nil {
		t.Fatal(err)
	}
	if ran || !strings.Contains(out.String(), "USAGE") {
		t.Errorf("-H first on a flagless leaf: ran=%v help=%q", ran, out.String())
	}
}

func TestCustomHelpPairHidesItsShortNotTheAppsH(t *testing.T) {
	root, out := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(fs *flag.FlagSet) {
		fs.Bool("help", false, "Show help")
		fs.Bool("H", false, "Show help")
		fs.String("h", "", "Host")
	})
	if err := root.Execute([]string{"run", "--out", "x"}); err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	lines := helpLines(out.String())
	if lines["-H,"] || lines["-H"] || !lines["-h"] {
		t.Errorf("want -H hidden and the app's -h shown:\n%s", out.String())
	}
}

func TestDisabledShortHelpIsAnUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": ""}},
		SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func([]string) error { return nil }}}}
	err := root.Execute([]string{"leaf", "x", "-h"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -h") {
		t.Errorf("error = %v", err)
	}
}

func TestUndefinedHWithCustomHelpAliasIsAnUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": "H"}},
		SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func([]string) error { return nil }}}}
	err := root.Execute([]string{"leaf", "x", "-h"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -h") {
		t.Errorf("error = %v", err)
	}
}

func TestSharedFlagSetStillHonoursHelp(t *testing.T) {
	shared := flag.NewFlagSet("shared", flag.ContinueOnError)
	shared.String("out", "", "Out path")
	ran := ""
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
		{Name: "a", Flags: shared, Run: func([]string) error { ran = "a"; return nil }},
		{Name: "b", Flags: shared, Run: func([]string) error { ran = "b"; return nil }},
	}}
	if err := root.Execute([]string{"b", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if ran != "" || !strings.Contains(out.String(), "USAGE") {
		t.Errorf("ran=%q, help printed=%v", ran, strings.Contains(out.String(), "USAGE"))
	}
}

func TestHelpRequestDoesNotCarryOverToTheNextExecute(t *testing.T) {
	for _, helpArg := range []string{"--help", "-h"} {
		t.Run(helpArg, func(t *testing.T) {
			ran := false
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			fs.String("out", "", "Out path")
			var out bytes.Buffer
			root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
				{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
			}}
			if err := root.Execute([]string{"run", "--out", "x", helpArg}); err != nil {
				t.Fatal(err)
			}
			if err := root.Execute([]string{"run", "--out", "y"}); err != nil {
				t.Fatal(err)
			}
			if !ran {
				t.Errorf("the earlier %s still counted on the next Execute", helpArg)
			}
		})
	}
}

func TestParallelTreesKeepSeparateHelpState(t *testing.T) {
	for _, name := range []string{"one", "two"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for range 50 {
				root, out := helpTree(&Config{}, func(*flag.FlagSet) {})
				if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), "USAGE") {
					t.Fatal("help not printed")
				}
			}
		})
	}
}

func TestHelpFlagsAddedByAnotherTreeStillCount(t *testing.T) {
	shared := flag.NewFlagSet("shared", flag.ContinueOnError)
	mk := func(alias string, out io.Writer, ran *bool) *Command {
		return &Command{Name: "app", Config: &Config{Out: out, FlagAliases: map[string]string{"help": alias}},
			SubCommands: []*Command{{Name: "leaf", Flags: shared, Run: func([]string) error { *ran = true; return nil }}}}
	}
	var ranA, ranB bool
	if err := mk("h", io.Discard, &ranA).Execute([]string{"leaf"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := mk("H", &out, &ranB).Execute([]string{"leaf", "x", "-h"}); err != nil {
		t.Fatal(err)
	}
	if ranB || !strings.Contains(out.String(), "USAGE") {
		t.Errorf("-h added by the other tree was ignored: ran=%v", ranB)
	}
}

func TestHelpRequestDoesNotCarryAcrossTrees(t *testing.T) {
	shared := flag.NewFlagSet("shared", flag.ContinueOnError)
	mk := func(alias string, ran *bool) *Command {
		return &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": alias}},
			SubCommands: []*Command{{Name: "leaf", Flags: shared, Run: func([]string) error { *ran = true; return nil }}}}
	}
	var ranA, ranB bool
	if err := mk("h", &ranA).Execute([]string{"leaf", "x", "-h"}); err != nil {
		t.Fatal(err)
	}
	if err := mk("H", &ranB).Execute([]string{"leaf", "y"}); err != nil {
		t.Fatal(err)
	}
	if ranA || !ranB {
		t.Errorf("ranA=%v ranB=%v, want only B to run", ranA, ranB)
	}
}
