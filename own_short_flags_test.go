package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestReviewTAppsOwnShortFlagsStayVisible(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.String("h", "", "Host to connect to")
	fs.Bool("v", false, "Verbose output")
	var out bytes.Buffer
	root := &Command{
		Name:        "app",
		Config:      &Config{Out: &out, HiddenFlags: map[string]bool{"debug": true}},
		SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}},
	}
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	root.SubCommands[0].PrintHelp()
	for _, want := range []string{"  -h string", "  -v\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q:\n%s", want, out.String())
		}
	}
	if !strings.Contains(zsh, "'-h[Host to connect to]:value:'") || !strings.Contains(zsh, "'-v[Verbose output]'") {
		t.Errorf("zsh lost the app's -h or -v:\n%s", zsh)
	}
	if !strings.Contains(bash, "    -h) return 0 ;;") {
		t.Errorf("bash does not know -h takes a value:\n%s", bash)
	}
}

func TestReviewTAppEntriesWinOverDefaults(t *testing.T) {
	cfg := (&Command{Name: "app", Config: &Config{FlagAliases: map[string]string{"help": "H"}}}).EffectiveConfig()
	if cfg.FlagAliases["help"] != "H" || cfg.FlagAliases["version"] != "v" {
		t.Errorf("FlagAliases = %v, want the app's help->H plus the default version->v", cfg.FlagAliases)
	}
}
