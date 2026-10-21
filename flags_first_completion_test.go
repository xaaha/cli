package cli

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func flagsFirstCompletionTree() *Command {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	fs.String("on", "", "Element id")
	return &Command{Name: "kapi", SubCommands: []*Command{{
		Name: "say", Aliases: []string{"s"}, Flags: fs, FlagsFirst: true,
		Args: []ArgDef{{Name: "page", Kind: KindFile}}, Run: func([]string) error { return nil },
	}}}
}

func TestD14ZshStopsCompletingFlagsAfterTheFirstPositional(t *testing.T) {
	zsh, _, err := tryScripts(flagsFirstCompletionTree())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "_kapi_say() {\n  _arguments -A '-*' \\") {
		t.Errorf("zsh say completer does not stop at the first positional:\n%s", zsh)
	}
	checkSyntax(t, "zsh", zsh)
}

func TestD14BashStopsCompletingFlagsAfterTheFirstPositional(t *testing.T) {
	bashBin := shellPath(t, "bash")
	var script bytes.Buffer
	if err := WriteBashCompletion(&script, flagsFirstCompletionTree()); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "kapi.bash")
	if err := os.WriteFile(scriptPath, script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	harness := `
source "$1"
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
_kapi
printf '%s\n' "${COMPREPLY[@]}" | sort | tr '\n' ' '
`
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"kapi", "say", "-"}, "--on "},
		{[]string{"kapi", "say", "page.html", "-"}, " "},
		{[]string{"kapi", "s", "page.html", "--on", ""}, "kapi.bash "},
	}
	for _, tc := range cases {
		args := append([]string{"-c", harness, "_", scriptPath}, tc.argv...)
		cmd := exec.Command(bashBin, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", tc.argv, err, out)
		}
		if got := string(out); got != tc.want {
			t.Errorf("%v: COMPREPLY = %q, want %q", tc.argv, got, tc.want)
		}
	}
	checkSyntax(t, "bash", script.String())
}
