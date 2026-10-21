package cli

import (
	"flag"
	"strings"
	"testing"
)

func TestReviewLZshLabelAndActionAreQuotedEverywhere(t *testing.T) {
	fs := flag.NewFlagSet("use", flag.ContinueOnError)
	fs.String("env", "", "Env")
	root := &Command{
		Name: "app",
		Config: &Config{
			FlagCompleters: map[string]string{"env": "env"},
			Completers: map[string]Completer{"env": {
				Label: `back\slash 'q': x`,
				Zsh:   `_values 'env' dev prod`,
				Bash:  func(string) string { return "COMPREPLY=()" },
			}},
		},
		SubCommands: []*Command{{Name: "use", Flags: fs, Args: []ArgDef{{Name: "env", Kind: "env"}}, Run: func([]string) error { return nil }}},
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	label := `back\\slash '\''q'\''\: x`
	action := `_values '\''env'\'' dev prod`
	for _, want := range []string{
		"'--env[Env]:" + label + ":" + action + "'",
		"'*:" + label + ":" + action + "'",
	} {
		if !strings.Contains(zsh, want) {
			t.Errorf("zsh missing %s\n%s", want, zsh)
		}
	}
	checkSyntax(t, "zsh", zsh)
}
