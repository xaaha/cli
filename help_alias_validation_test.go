package cli

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestReviewAEInvalidHelpAliasIsAnError(t *testing.T) {
	for _, alias := range []string{"-x", "a=b", "a b"} {
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
