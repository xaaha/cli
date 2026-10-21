package cli

import (
	"flag"
	"strings"
	"testing"
)

func TestReviewEAliasPairSharesItsCompleter(t *testing.T) {
	for _, registered := range []string{"out", "o"} {
		t.Run("registered on "+registered, func(t *testing.T) {
			fs := flag.NewFlagSet("save", flag.ContinueOnError)
			fs.String("out", "", "Out path")
			fs.String("o", "", "Out path")
			root := &Command{
				Name: "app",
				Config: &Config{
					FlagAliases:    map[string]string{"out": "o"},
					FlagCompleters: map[string]string{registered: KindFile},
				},
				SubCommands: []*Command{{Name: "save", Flags: fs, Run: func([]string) error { return nil }}},
			}
			zsh, bash, err := tryScripts(root)
			if err != nil {
				t.Fatal(err)
			}
			if want := `'(--out -o)'{--out,-o}'[Out path]:path:_files'`; !strings.Contains(zsh, want) {
				t.Errorf("zsh missing %s\n%s", want, zsh)
			}
			if want := `--out|-o|-out) _app_path_files "$2" ;;`; !strings.Contains(bash, want) {
				t.Errorf("bash missing %s\n%s", want, bash)
			}
		})
	}
}
