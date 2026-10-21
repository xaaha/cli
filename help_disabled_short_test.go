package cli

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestReviewAGDisabledShortHelpIsAnUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": ""}},
		SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func([]string) error { return nil }}}}
	err := root.Execute([]string{"leaf", "x", "-h"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -h") {
		t.Errorf("error = %v", err)
	}
}

func TestReviewAMUndefinedHWithCustomAliasIsAnUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, FlagAliases: map[string]string{"help": "H"}},
		SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func([]string) error { return nil }}}}
	err := root.Execute([]string{"leaf", "x", "-h"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -h") {
		t.Errorf("error = %v", err)
	}
}
