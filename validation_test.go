package cli

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestInvalidHelpAliasIsAnError(t *testing.T) {
	for _, alias := range []string{"-x", "a=b", "a b", "a'b", "a;b", "a$b"} {
		mk := func() *Command {
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			return &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": alias}},
				SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}}}
		}
		if err := mk().Execute([]string{"run"}); err == nil || !strings.Contains(err.Error(), alias) {
			t.Errorf("Execute with help alias %q: error = %v", alias, err)
		}
		if err := WriteZshCompletion(io.Discard, mk()); err == nil {
			t.Errorf("WriteZshCompletion with help alias %q: no error", alias)
		}
		if err := WriteBashCompletion(io.Discard, mk()); err == nil {
			t.Errorf("WriteBashCompletion with help alias %q: no error", alias)
		}
	}
}

func TestHelpAliasIsCheckedWithoutAnyFlagSet(t *testing.T) {
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": "-x"}},
		SubCommands: []*Command{{Name: "run", Run: func([]string) error { return nil }}}}
	if err := root.Execute([]string{"run"}); err == nil {
		t.Error("invalid help alias accepted on a tree with no FlagSets")
	}
}

func TestUnsafeNamesAreRejected(t *testing.T) {
	noop := func([]string) error { return nil }
	badFlag := flag.NewFlagSet("go", flag.ContinueOnError)
	badFlag.String("a$b", "", "x")
	cases := map[string]*Command{
		"root name":    {Name: "my app", Run: noop},
		"command name": {Name: "app", SubCommands: []*Command{{Name: "go'", Run: noop}}},
		"alias":        {Name: "app", SubCommands: []*Command{{Name: "go", Aliases: []string{"$(x)"}, Run: noop}}},
		"flag name":    {Name: "app", SubCommands: []*Command{{Name: "go", Flags: badFlag, Run: noop}}},
	}
	for name, tree := range cases {
		t.Run(name, func(t *testing.T) {
			for shell, write := range map[string]func(*bytes.Buffer, *Command) error{
				"zsh":  func(b *bytes.Buffer, r *Command) error { return WriteZshCompletion(b, r) },
				"bash": func(b *bytes.Buffer, r *Command) error { return WriteBashCompletion(b, r) },
			} {
				var b bytes.Buffer
				if err := write(&b, tree); err == nil || !strings.Contains(err.Error(), "cannot be used in a completion script") {
					t.Errorf("%s: error = %v, want a rejected name", shell, err)
				}
			}
		})
	}
}

func TestNameRuleEdgeCases(t *testing.T) {
	for _, bad := range []string{"", "-x", ".x", "_x"} {
		root := &Command{Name: "app", SubCommands: []*Command{{Name: bad, Run: func([]string) error { return nil }}}}
		if _, _, err := tryScripts(root); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	hidden := flag.NewFlagSet("go", flag.ContinueOnError)
	hidden.String("a b", "", "internal")
	root := &Command{
		Name:        "app",
		Config:      &Config{HiddenFlags: map[string]bool{"a b": true}},
		SubCommands: []*Command{{Name: "go", Flags: hidden, Run: func([]string) error { return nil }}},
	}
	if _, _, err := tryScripts(root); err != nil {
		t.Errorf("hidden flag should not be validated: %v", err)
	}
}

func TestHiddenCommandsAreNotValidated(t *testing.T) {
	root := &Command{Name: "app", SubCommands: []*Command{{Name: "go now", Hidden: true, Run: func([]string) error { return nil }}}}
	if _, _, err := tryScripts(root); err != nil {
		t.Fatalf("hidden command should be ignored: %v", err)
	}
}

func TestUnknownCompleterIsAnError(t *testing.T) {
	root := &Command{Name: "kapi", SubCommands: []*Command{
		{Name: "say", Args: []ArgDef{{Name: "page", Kind: "nope"}}, Run: func([]string) error { return nil }},
	}}
	for name, write := range map[string]func(*bytes.Buffer, *Command) error{
		"zsh":  func(b *bytes.Buffer, r *Command) error { return WriteZshCompletion(b, r) },
		"bash": func(b *bytes.Buffer, r *Command) error { return WriteBashCompletion(b, r) },
	} {
		var b bytes.Buffer
		err := write(&b, root)
		if err == nil || !strings.Contains(err.Error(), `"nope"`) {
			t.Errorf("%s: error = %v, want unknown completer \"nope\"", name, err)
		}
	}
}

func TestIncompleteCompletersAreErrors(t *testing.T) {
	treeWith := func(kind string, c map[string]Completer) *Command {
		fs := flag.NewFlagSet("say", flag.ContinueOnError)
		fs.String("on", "", "Element id")
		return &Command{
			Name:        "kapi",
			Config:      &Config{FlagCompleters: map[string]string{"on": kind}, Completers: c},
			SubCommands: []*Command{{Name: "say", Flags: fs, Run: func([]string) error { return nil }}},
		}
	}
	bash := func(string) string { return "COMPREPLY=()" }
	cases := []struct {
		name string
		tree *Command
		want string
	}{
		{"unknown flag kind", treeWith("nope", nil), `unknown completer "nope"`},
		{"no bash command", treeWith("el", map[string]Completer{"el": {Label: "el", Zsh: "_el"}}), `completer "el" needs Zsh and Bash`},
		{"no zsh action", treeWith("el", map[string]Completer{"el": {Label: "el", Bash: bash}}), `completer "el" needs Zsh and Bash`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tryScripts(tc.tree)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBoolFlagsAreNotCheckedForCompleters(t *testing.T) {
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
