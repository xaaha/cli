package cli

import (
	"bytes"
	"flag"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestExecuteDispatchesToSubcommand(t *testing.T) {
	called := ""
	root := &Command{
		Name: "root",
		SubCommands: []*Command{
			{
				Name:  "sub1",
				Short: "first sub",
				Run: func(_ []string) error {
					called = "sub1"
					return nil
				},
			},
			{
				Name:  "sub2",
				Short: "second sub",
				Run: func(_ []string) error {
					called = "sub2"
					return nil
				},
			},
		},
	}

	if err := root.Execute([]string{"sub2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != "sub2" {
		t.Errorf("expected sub2 to be called, got %q", called)
	}
}

func TestExecuteMatchesAliases(t *testing.T) {
	called := false
	root := &Command{
		Name: "root",
		SubCommands: []*Command{
			{
				Name:    "gql",
				Aliases: []string{"graphql", "GraphQL"},
				Short:   "GraphQL explorer",
				Run: func(_ []string) error {
					called = true
					return nil
				},
			},
		},
	}

	tests := []string{"gql", "graphql", "GraphQL"}
	for _, name := range tests {
		called = false
		if err := root.Execute([]string{name}); err != nil {
			t.Fatalf("unexpected error for %q: %v", name, err)
		}
		if !called {
			t.Errorf("expected gql handler to be called for alias %q", name)
		}
	}
}

func TestExecutePassesArgsToRun(t *testing.T) {
	var gotArgs []string
	root := &Command{
		Name: "root",
		SubCommands: []*Command{
			{
				Name: "migrate",
				Run: func(args []string) error {
					gotArgs = args
					return nil
				},
			},
		},
	}

	if err := root.Execute([]string{"migrate", "file1.json", "file2.json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "file1.json" || gotArgs[1] != "file2.json" {
		t.Errorf("expected [file1.json file2.json], got %v", gotArgs)
	}
}

func TestExecuteWalksNestedSubcommands(t *testing.T) {
	var gotArgs []string
	root := &Command{
		Name: "root",
		SubCommands: []*Command{
			{
				Name: "env",
				SubCommands: []*Command{
					{
						Name: "set",
						Run: func(args []string) error {
							gotArgs = args
							return nil
						},
					},
				},
			},
		},
	}

	// root → env → set with remaining args
	if err := root.Execute([]string{"env", "set", "API_KEY", "--env", "prod"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "API_KEY" {
		t.Errorf("expected [API_KEY --env prod], got %v", gotArgs)
	}

	// help at the middle level should not reach the leaf
	gotArgs = nil
	if err := root.Execute([]string{"env", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotArgs != nil {
		t.Error("set.Run should not be called when help is requested on parent")
	}
}

func TestExecuteNoArgsOnGroupPrintsHelp(t *testing.T) {
	// command with no Run and no args should print help without error
	cmd := &Command{
		Name: "root",
		Long: "Root command",
		SubCommands: []*Command{
			{Name: "sub1", Short: "first"},
		},
	}

	if err := cmd.Execute(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteNoArgsCallsRun(t *testing.T) {
	called := false
	cmd := &Command{
		Name: "root",
		Run: func(_ []string) error {
			called = true
			return nil
		},
	}

	if err := cmd.Execute(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("Run should be called when no args and Run is set")
	}
}

func TestFindSubMatchesNameOrAlias(t *testing.T) {
	sub := &Command{
		Name:    "gql",
		Aliases: []string{"graphql"},
	}
	root := &Command{
		Name:        "root",
		SubCommands: []*Command{sub},
	}

	tests := []struct {
		name  string
		input string
		found bool
	}{
		{"by name", "gql", true},
		{"by alias", "graphql", true},
		{"not found", "unknown", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := root.FindSub(tc.input)
			if tc.found && result == nil {
				t.Errorf("expected to find subcommand for %q", tc.input)
			}
			if !tc.found && result != nil {
				t.Errorf("expected nil for %q, got %v", tc.input, result)
			}
		})
	}
}

func TestBoolFlagsDoNotSwallowTheSubcommand(t *testing.T) {
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

func TestDoubleDashStopsSubcommandDispatch(t *testing.T) {
	var rootArgs []string
	statusRan := false
	root := &Command{Name: "app", Config: &Config{Out: io.Discard},
		Run:         func(args []string) error { rootArgs = args; return nil },
		SubCommands: []*Command{{Name: "status", Run: func([]string) error { statusRan = true; return nil }}},
	}
	for _, args := range [][]string{{"--", "status"}, {"--", "x", "status"}} {
		statusRan, rootArgs = false, nil
		if err := root.Execute(args); err != nil {
			t.Fatal(err)
		}
		if statusRan || !slices.Equal(rootArgs, args) {
			t.Errorf("%v: statusRan=%v rootArgs=%v, want the flagless root to get its args unchanged", args, statusRan, rootArgs)
		}
	}
}

func TestLoneDashIsAPositionalNotAFlag(t *testing.T) {
	ran := false
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, Err: io.Discard},
		SubCommands: []*Command{{Name: "status", Run: func([]string) error { ran = true; return nil }}},
	}
	err := root.Execute([]string{"-", "status"})
	if ran || err == nil || !strings.Contains(err.Error(), `unknown command "-"`) {
		t.Errorf("ran=%v err=%v, want unknown command \"-\"", ran, err)
	}
}

func TestUnknownCommandErrorNamesFullPath(t *testing.T) {
	var out bytes.Buffer
	root, _, _ := linkTree(&out)
	err := root.Execute([]string{"grp", "bogus"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "bogus" for app grp`) {
		t.Fatalf("error = %v", err)
	}
}
