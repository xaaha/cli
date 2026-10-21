package cli

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZshFunctionNamesMustNotCollide(t *testing.T) {
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

func TestZshOnlyEmittedFunctionsCanCollide(t *testing.T) {
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

func TestZshCompleterLabelIsEscaped(t *testing.T) {
	fs := flag.NewFlagSet("go", flag.ContinueOnError)
	fs.String("env", "", "Env")
	root := &Command{
		Name: "app",
		Config: &Config{
			FlagCompleters: map[string]string{"env": "env"},
			Completers: map[string]Completer{"env": {
				Label: "user's env: name",
				Zsh:   "_app_envs",
				Bash:  func(string) string { return "COMPREPLY=()" },
			}},
		},
		SubCommands: []*Command{{Name: "go", Flags: fs, Run: func([]string) error { return nil }}},
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, `:user'\''s env\: name:_app_envs`) {
		t.Errorf("label not escaped:\n%s", zsh)
	}
	checkSyntax(t, "zsh", zsh)
}

func TestZshLabelAndActionAreQuotedEverywhere(t *testing.T) {
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

func TestZshActionKeepsBackslashAndColon(t *testing.T) {
	fs := flag.NewFlagSet("use", flag.ContinueOnError)
	fs.String("env", "", "Env")
	root := &Command{
		Name: "app",
		Config: &Config{
			FlagCompleters: map[string]string{"env": "env"},
			Completers: map[string]Completer{"env": {
				Label: "env", Zsh: `((dev\:Development))`,
				Bash: func(string) string { return "COMPREPLY=()" },
			}},
		},
		SubCommands: []*Command{{Name: "use", Flags: fs, Run: func([]string) error { return nil }}},
	}
	zsh, _, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := `'--env[Env]:env:((dev\:Development))'`; !strings.Contains(zsh, want) {
		t.Errorf("zsh missing %s\n%s", want, zsh)
	}
}

func TestZshFlagsFirstStopsCompletingFlagsAfterTheFirstPositional(t *testing.T) {
	zsh, _, err := tryScripts(flagsFirstCompletionTree())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "_kapi_say() {\n  _arguments -A '-*' \\") {
		t.Errorf("zsh say completer does not stop at the first positional:\n%s", zsh)
	}
	checkSyntax(t, "zsh", zsh)
}

func TestZshFirstTabCompletesAfterFpathInstall(t *testing.T) {
	zshBin := shellPath(t, "zsh")
	root := &Command{Name: "app", SubCommands: []*Command{{Name: "greet", Short: "Say hello", Run: func([]string) error { return nil }}}}
	var script bytes.Buffer
	if err := WriteZshCompletion(&script, root); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "fn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fn", "_app"), script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	harness := `
zmodload zsh/zpty || exit 2
zpty z "HOME=$1 zsh -f -i"
zpty -w z "PS1='%% '; fpath=($1/fn \$fpath); autoload -Uz compinit; compinit -u -d $1/zcompdump; print READY"
zpty -r -m z line '*READY*' || exit 3
zpty -w -n z $'app gr\t'
out=
for i in {1..50}; do
  zpty -r -t z chunk && out+=$chunk
  [[ $out == *greet* ]] && break
  sleep 0.1
done
zpty -d z
print -r -- $out
`
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, zshBin, "-f", "-c", harness, "_", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("zsh harness: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "app greet") {
		t.Errorf("first TAB after an fpath install did not complete greet:\n%q", out)
	}
}
