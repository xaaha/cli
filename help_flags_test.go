package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func helpTree(cfg *Config, register func(*flag.FlagSet)) (*Command, *bytes.Buffer) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.String("out", "", "Out path")
	register(fs)
	var out bytes.Buffer
	cfg.Out = &out
	ran := &Command{Name: "run", Flags: fs, Run: func([]string) error { return nil }}
	return &Command{Name: "app", Config: cfg, SubCommands: []*Command{ran}}, &out
}

func helpLines(help string) map[string]bool {
	lines := map[string]bool{}
	for _, l := range strings.Split(help, "\n") {
		if strings.HasPrefix(l, "  -") {
			lines[strings.Fields(l)[0]] = true
		}
	}
	return lines
}

func TestReviewWOnlyInjectedHelpFlagsAreHidden(t *testing.T) {
	root, out := helpTree(&Config{}, func(fs *flag.FlagSet) { fs.String("h", "", "Host") })
	if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	lines := helpLines(out.String())
	if !lines["-h"] || lines["-help"] {
		t.Errorf("want the app's -h listed and the injected --help hidden:\n%s", out.String())
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'-h[Host]:value:'") {
		t.Errorf("zsh lost the app's -h after Execute:\n%s", zsh)
	}
}

func TestReviewWCustomShortHelpAlias(t *testing.T) {
	root, out := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(*flag.FlagSet) {})
	if err := root.Execute([]string{"run", "--out", "x", "-H"}); err != nil {
		t.Fatalf("-H should request help: %v", err)
	}
	if !strings.Contains(out.String(), "FLAGS") {
		t.Fatalf("-H did not print help:\n%s", out.String())
	}
	if lines := helpLines(out.String()); lines["-h"] || lines["-help"] || lines["-H,"] {
		t.Errorf("injected help flags listed:\n%s", out.String())
	}
}

func TestReviewWExplicitHiddenFalseShowsInjectedHelp(t *testing.T) {
	root, out := helpTree(&Config{HiddenFlags: map[string]bool{"h": false}}, func(*flag.FlagSet) {})
	if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "-h, --help") {
		t.Errorf("explicit HiddenFlags h=false should list -h, --help:\n%s", out.String())
	}
}

func TestReviewWCustomAliasPairHidesItsShortNotTheAppsH(t *testing.T) {
	root, out := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(fs *flag.FlagSet) {
		fs.Bool("help", false, "Show help")
		fs.Bool("H", false, "Show help")
		fs.String("h", "", "Host")
	})
	if err := root.Execute([]string{"run", "--out", "x"}); err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	lines := helpLines(out.String())
	if lines["-H,"] || lines["-H"] || !lines["-h"] {
		t.Errorf("want -H hidden and the app's -h shown:\n%s", out.String())
	}
}
