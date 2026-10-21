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
COMP_LINE=${CLI_TEST_LINE:-}
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
line=keep k=keep
_hulak
[[ $line == keep && $k == keep ]] || echo "LEAKED line=$line k=$k"
printf '%s\n' "${COMPREPLY[@]}" | sort | tr '\n' ' '
`
	cases := []struct {
		name string
		argv []string
		want string
		line string
	}{
		{"subcommands", []string{"hulak", "secrets", "keys", ""}, "get set ", ""},
		{"alias path", []string{"hulak", "env", "keys", "g"}, "get ", ""},
		{"nested aliases and flags", []string{"hulak", "init", "cl"}, "classic ", ""},
		{"skips a flag value, then yaml files", []string{"hulak", "run", "--timeout", "5", ""}, "a.yaml sub ", ""},
		{"flag value is not taken as a subcommand", []string{"hulak", "--env", "keys", "secrets", ""}, "keys list ", ""},
		{"flag value completer", []string{"hulak", "run", "--out", "no"}, "notes.txt ", ""},
		{"--flag=value before a subcommand", []string{"hulak", "--env", "=", "prod", "secrets", ""}, "keys list ", "hulak --env=prod secrets "},
		{"value after --flag=", []string{"hulak", "run", "--out", "=", "no"}, "notes.txt ", "hulak run --out=no"},
		{"an empty --flag= value does not take the next word", []string{"hulak", "--env", "=", "secrets", ""}, "keys list ", "hulak --env= secrets "},
		{"a later empty --flag= does not affect an earlier one", []string{"hulak", "--env", "=", "prod", "secrets", "--env", "=", "keys", ""}, "get set ", "hulak --env=prod secrets --env= keys "},
		{"without COMP_LINE a value after = is assumed", []string{"hulak", "--env", "=", "prod", "secrets", ""}, "keys list ", ""},
		{"a COMP_LINE that does not match the words assumes a value", []string{"hulak", "--env", "=", "prod", "secrets", ""}, "keys list ", "hulak = secrets "},
		{"a value starting with = after --flag==", []string{"hulak", "--env", "==", "prod", "secrets", ""}, "keys list ", "hulak --env==prod secrets "},
		{"value after --flag==", []string{"hulak", "run", "--out", "==", "no"}, "notes.txt ", "hulak run --out==no"},
		{"a space after --flag= starts a new word", []string{"hulak", "--env", "=", ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version ", "hulak --env= "},
		{"a space after --flag== starts a new word", []string{"hulak", "--env", "==", ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version ", "hulak --env== "},
		{"a quoted value containing = after a bool flag is a positional", []string{"hulak", "--debug", `"a=b"`, "secrets", ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version ", `hulak --debug "a=b" secrets `},
		{"a glob-like word does not act as a pattern", []string{"hulak", "-*", "=", "secrets", ""}, "keys list ", "hulak -*= secrets "},
		{"an empty value next to a filled one", []string{"hulak", "--fp", "=", "--env", "=", "prod", "secrets", ""}, "keys list ", "hulak --fp= --env=prod secrets "},
		{"-- after a positional still counts", []string{"hulak", "run", "x", "--", "-"}, " ", ""},
		{"after -- the root completes nothing", []string{"hulak", "--", "secrets", ""}, " ", ""},
		{"after -- only the command's positional completer runs", []string{"hulak", "run", "--", ""}, "a.yaml sub ", ""},
		{"after -- a flag-looking word takes no value", []string{"hulak", "run", "--", "--out", ""}, "a.yaml sub ", ""},
		{"a lone - is not a flag", []string{"hulak", "-", "secrets", ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-c", harness, "_", scriptPath}, tc.argv...)
			cmd := exec.Command(bashBin, args...)
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "CLI_TEST_LINE="+tc.line)
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
