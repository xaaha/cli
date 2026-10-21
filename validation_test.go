package cli

import (
	"flag"
	"strings"
	"testing"
)

func TestReviewCIncompleteCompletersAreErrors(t *testing.T) {
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
