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

func TestD14FlagsFirstNestedUnderFlagsFirstParent(t *testing.T) {
	bashBin := shellPath(t, "bash")
	leaf := flag.NewFlagSet("say", flag.ContinueOnError)
	leaf.String("on", "", "Element id")
	parent := flag.NewFlagSet("page", flag.ContinueOnError)
	parent.String("as", "", "As")
	root := &Command{Name: "kapi", SubCommands: []*Command{{
		Name: "page", Flags: parent, FlagsFirst: true, Args: []ArgDef{{Name: "p", Kind: KindFile}}, Run: func([]string) error { return nil },
		SubCommands: []*Command{{Name: "say", Flags: leaf, FlagsFirst: true, Args: []ArgDef{{Name: "p", Kind: KindFile}}, Run: func([]string) error { return nil }}},
	}}}
	var script bytes.Buffer
	if err := WriteBashCompletion(&script, root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "kapi.bash")
	if err := os.WriteFile(path, script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	harness := `
source "$1"
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
_kapi
printf '%s\n' "${COMPREPLY[@]}" | sort | tr '\n' ' '
`
	for _, argv := range [][]string{{"kapi", "page", "say", "x.html", "-"}, {"kapi", "page", "x.html", "-"}} {
		args := append([]string{"-c", harness, "_", path}, argv...)
		out, err := exec.Command(bashBin, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", argv, err, out)
		}
		if got := string(out); got != " " {
			t.Errorf("%v: COMPREPLY = %q, want no flags after the first positional", argv, got)
		}
	}
}
