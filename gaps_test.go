package cli

import (
	"bytes"
	"flag"
	"io"
	"os"
	"testing"
)

func TestReviewQAutoColorUsesConfiguredWriter(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	t.Setenv("NO_COLOR", "")
	if !(&Config{Out: devNull}).useColor() {
		t.Error("a character device as Out should get colour in auto mode")
	}
	if (&Config{Out: &bytes.Buffer{}}).useColor() {
		t.Error("a buffer as Out must not get colour in auto mode")
	}
}

func TestReviewQDefaultWriters(t *testing.T) {
	cfg := (&Command{Name: "app"}).EffectiveConfig()
	if cfg.Out != os.Stdout || cfg.Err != os.Stderr {
		t.Errorf("defaults: Out=%v Err=%v", cfg.Out, cfg.Err)
	}
}

func TestReviewQBothWritersLinkGrandchildren(t *testing.T) {
	for shell, write := range map[string]func(io.Writer, *Command) error{"zsh": WriteZshCompletion, "bash": WriteBashCompletion} {
		var out bytes.Buffer
		leaf := &Command{Name: "leaf", Run: func([]string) error { return nil }}
		root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{{Name: "grp", SubCommands: []*Command{leaf}}}}
		if err := write(io.Discard, root); err != nil {
			t.Fatal(err)
		}
		if leaf.EffectiveConfig().Out != &out {
			t.Errorf("%s writer did not link the grandchild to the root Config", shell)
		}
	}
}

func TestReviewQNameRuleEdges(t *testing.T) {
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
