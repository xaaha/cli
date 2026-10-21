package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewHGeneratedBashFunctionCompletes(t *testing.T) {
	bashBin := shellPath(t, "bash")
	var script bytes.Buffer
	if err := WriteBashCompletion(&script, fixtureTree()); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "hulak.bash")
	if err := os.WriteFile(scriptPath, script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	for _, p := range []string{"work/sub", "work/a.yaml", "work/notes.txt"} {
		full := filepath.Join(dir, p)
		var err error
		if strings.HasSuffix(p, "sub") {
			err = os.MkdirAll(full, 0o755)
		} else {
			err = os.WriteFile(full, nil, 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	harness := `
source "$1"
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
_hulak
printf '%s\n' "${COMPREPLY[@]}" | sort | tr '\n' ' '
`
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"subcommands", []string{"hulak", "secrets", "keys", ""}, "get set "},
		{"alias path", []string{"hulak", "env", "keys", "g"}, "get "},
		{"nested aliases and flags", []string{"hulak", "init", "cl"}, "classic "},
		{"skips a flag value, then yaml files", []string{"hulak", "run", "--timeout", "5", ""}, "a.yaml sub "},
		{"flag value is not taken as a subcommand", []string{"hulak", "--env", "keys", "secrets", ""}, "keys list "},
		{"flag value completer", []string{"hulak", "run", "--out", "no"}, "notes.txt "},
		{"-- stops the command walk", []string{"hulak", "--", "secrets", ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version "},
		{"a lone - is not a flag", []string{"hulak", "-", "secrets", ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-c", harness, "_", scriptPath}, tc.argv...)
			cmd := exec.Command(bashBin, args...)
			cmd.Dir = work
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("bash failed: %v\n%s", err, out)
			}
			if got := string(out); got != tc.want {
				t.Errorf("COMPREPLY = %q, want %q (argv=%v)", got, tc.want, tc.argv)
			}
		})
	}
}
