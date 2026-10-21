package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func linkTree(out *bytes.Buffer) (root, grp, leaf *Command) {
	leaf = &Command{Name: "leaf", Short: "A leaf", Run: func([]string) error { return nil }}
	grp = &Command{Name: "grp", Short: "A group", SubCommands: []*Command{leaf}}
	other := &Command{Name: "other", Short: "Another", Run: func([]string) error { return nil }}
	root = &Command{Name: "app", Config: &Config{Out: out, Err: io.Discard}, SubCommands: []*Command{grp, other}}
	return root, grp, leaf
}

func TestReviewDDirectHelpAfterExecuteUsesRoot(t *testing.T) {
	var out bytes.Buffer
	root, _, leaf := linkTree(&out)
	if err := root.Execute([]string{"other"}); err != nil {
		t.Fatal(err)
	}
	leaf.PrintHelp()
	if !strings.Contains(out.String(), "Use `app <command> --help`") {
		t.Errorf("leaf help did not use the root's name and writer: %q", out.String())
	}
}

func TestReviewDCompletionWriterLinksTree(t *testing.T) {
	var out bytes.Buffer
	root, grp, _ := linkTree(&out)
	if err := WriteZshCompletion(io.Discard, root); err != nil {
		t.Fatal(err)
	}
	if grp.EffectiveConfig().Out != &out {
		t.Error("subcommand does not see the root Config after a completion writer ran")
	}
}

func TestReviewDUnknownCommandNamesFullPath(t *testing.T) {
	var out bytes.Buffer
	root, _, _ := linkTree(&out)
	err := root.Execute([]string{"grp", "bogus"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "bogus" for app grp`) {
		t.Fatalf("error = %v", err)
	}
}
