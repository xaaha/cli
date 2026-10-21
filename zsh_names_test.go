package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestReviewKZshFunctionNamesMustNotCollide(t *testing.T) {
	leaf := func(name string) *Command {
		fs := flag.NewFlagSet(name, flag.ContinueOnError)
		fs.String("out", "", "Out")
		return &Command{Name: name, Flags: fs, Run: func([]string) error { return nil }}
	}
	group := func(name string, subs ...*Command) *Command { return &Command{Name: name, SubCommands: subs} }
	cases := map[string]*Command{
		"child named subs":    group("app", leaf("subs"), leaf("other")),
		"dash and underscore": group("app", leaf("a-b"), leaf("a_b")),
		"across levels":       group("app", group("x", leaf("y_z")), group("x_y", leaf("z"))),
	}
	for name, root := range cases {
		t.Run(name, func(t *testing.T) {
			err := WriteZshCompletion(&bytes.Buffer{}, root)
			if err == nil || !strings.Contains(err.Error(), "zsh function") {
				t.Fatalf("error = %v, want a zsh function name collision", err)
			}
		})
	}
}
