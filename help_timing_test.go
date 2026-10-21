package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestReviewZAppsOwnShortFlagWorksAsFirstArgument(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	host := fs.String("h", "", "Host")
	ran := false
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
		{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
	}}
	if err := root.Execute([]string{"run", "-h", "example.com"}); err != nil {
		t.Fatal(err)
	}
	if !ran || *host != "example.com" || out.Len() != 0 {
		t.Errorf("ran=%v host=%q help=%q", ran, *host, out.String())
	}
}

func TestReviewZShownHelpFlagsDoNotDependOnArgumentOrder(t *testing.T) {
	for _, args := range [][]string{{"run", "--help"}, {"run", "--out", "x", "--help"}} {
		root, out := helpTree(&Config{HiddenFlags: map[string]bool{"h": false}}, func(*flag.FlagSet) {})
		if err := root.Execute(args); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "-h, --help") {
			t.Errorf("%v: -h, --help not listed:\n%s", args, out.String())
		}
	}
	root, _ := helpTree(&Config{HiddenFlags: map[string]bool{"h": false}}, func(*flag.FlagSet) {})
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "{--help,-h}") {
		t.Errorf("a fresh tree's zsh script does not offer the shown help flags:\n%s", zsh)
	}
}

func TestReviewZCustomShortHelpReplacesH(t *testing.T) {
	root, _ := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(*flag.FlagSet) {})
	if err := root.Execute([]string{"run", "--out", "x", "-h"}); err == nil || !strings.Contains(err.Error(), "-h") {
		t.Errorf("-h with help->H: error = %v, want an unknown flag", err)
	}

	ran := false
	var out bytes.Buffer
	bare := &Command{Name: "app", Config: &Config{Out: &out, FlagAliases: map[string]string{"help": "H"}}, SubCommands: []*Command{
		{Name: "run", Run: func([]string) error { ran = true; return nil }},
	}}
	if err := bare.Execute([]string{"run", "-H"}); err != nil {
		t.Fatal(err)
	}
	if ran || !strings.Contains(out.String(), "LEARN MORE") {
		t.Errorf("-H first on a flagless leaf: ran=%v help=%q", ran, out.String())
	}
}

func TestReviewZAppsOwnLongHelpIsNotPairedWithInjectedShort(t *testing.T) {
	root, out := helpTree(&Config{}, func(fs *flag.FlagSet) { fs.Bool("help", false, "Show the manual") })
	if err := root.Execute([]string{"run", "--out", "x"}); err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	lines := helpLines(out.String())
	if !lines["-help"] || lines["-h"] || lines["-h,"] {
		t.Errorf("want the app's -help listed and no injected -h:\n%s", out.String())
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'--help[Show the manual]'") || strings.Contains(zsh, "-h[") {
		t.Errorf("zsh should offer only the app's --help:\n%s", zsh)
	}
}

func TestReviewZInjectedFlagsStayOutOfCompletion(t *testing.T) {
	root, _ := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(fs *flag.FlagSet) { fs.String("h", "", "Host") })
	if err := root.Execute([]string{"run", "--out", "x"}); err != nil {
		t.Fatal(err)
	}
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(zsh, "--help") || strings.Contains(zsh, "-H[") || strings.Contains(bash, "--help") || strings.Contains(bash, " -H") {
		t.Errorf("injected help flags leaked into completion:\nzsh:\n%s\nbash:\n%s", zsh, bash)
	}
}
