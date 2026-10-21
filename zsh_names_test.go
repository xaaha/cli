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
		"positional-only leaf": group("app",
			&Command{Name: "a-b", Args: []ArgDef{{Name: "f", Kind: KindFile}}, Run: func([]string) error { return nil }},
			leaf("a_b")),
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

func TestReviewUOnlyEmittedFunctionsCanCollide(t *testing.T) {
	withFlag := flag.NewFlagSet("x_y", flag.ContinueOnError)
	withFlag.String("out", "", "Out")
	root := &Command{Name: "app", SubCommands: []*Command{
		{Name: "x-y", Run: func([]string) error { return nil }},
		{Name: "x_y", Flags: withFlag, Run: func([]string) error { return nil }},
	}}
	if err := WriteZshCompletion(&bytes.Buffer{}, root); err != nil {
		t.Errorf("x-y emits no zsh function, so nothing collides: %v", err)
	}
}

func TestReviewUBashIgnoresZshNameCollisions(t *testing.T) {
	a := flag.NewFlagSet("a-b", flag.ContinueOnError)
	a.String("x", "", "x")
	b := flag.NewFlagSet("a_b", flag.ContinueOnError)
	b.String("y", "", "y")
	root := &Command{Name: "app", SubCommands: []*Command{
		{Name: "a-b", Flags: a, Run: func([]string) error { return nil }},
		{Name: "a_b", Flags: b, Run: func([]string) error { return nil }},
	}}
	if err := WriteBashCompletion(&bytes.Buffer{}, root); err != nil {
		t.Errorf("bash has no per-command functions: %v", err)
	}
}
