package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func oneFlagTree(cfg *Config, name, usage string) *Command {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.String(name, "", usage)
	return &Command{Name: "app", Config: cfg, SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}}}
}

func TestReviewMLongFlagWithoutItsShortPartnerIsShown(t *testing.T) {
	var out bytes.Buffer
	root := oneFlagTree(&Config{Out: &out, FlagAliases: map[string]string{"output": "o"}}, "output", "Output file")
	if err := root.Execute([]string{"run", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "  -output string") {
		t.Errorf("--output missing from help:\n%s", out.String())
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'--output[Output file]:value:'") {
		t.Errorf("--output missing from zsh:\n%s", zsh)
	}
}

func TestReviewMCompleterNeedsBothHalvesRegistered(t *testing.T) {
	root := oneFlagTree(&Config{
		FlagAliases:    map[string]string{"env": "e"},
		FlagCompleters: map[string]string{"env": KindDir},
	}, "e", "Exclude pattern")
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'-e[Exclude pattern]:value:'") {
		t.Errorf("-e borrowed --env's completer in zsh:\n%s", zsh)
	}
	if strings.Contains(bash, "_app_dir_files") {
		t.Errorf("-e borrowed --env's completer in bash:\n%s", bash)
	}
}
