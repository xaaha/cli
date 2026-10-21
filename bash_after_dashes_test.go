package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReviewAXAfterDashesReachesNestedAndAliasedCommands(t *testing.T) {
	bashBin := shellPath(t, "bash")
	root := &Command{Name: "app", SubCommands: []*Command{{
		Name: "secrets", Aliases: []string{"env"}, SubCommands: []*Command{{
			Name: "load", Aliases: []string{"ld"}, Args: []ArgDef{{Name: "file", Kind: KindFile}},
			Run: func([]string) error { return nil },
		}},
	}}}
	var script bytes.Buffer
	if err := WriteBashCompletion(&script, root); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "app.bash")
	if err := os.WriteFile(scriptPath, script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vars.env"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	harness := `
source "$1"
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
_app
printf '%s\n' "${COMPREPLY[@]}" | sort | tr '\n' ' '
`
	for _, argv := range [][]string{
		{"app", "secrets", "load", "--", ""},
		{"app", "env", "ld", "--", ""},
	} {
		args := append([]string{"-c", harness, "_", scriptPath}, argv...)
		cmd := exec.Command(bashBin, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", argv, err, out)
		}
		if got := string(out); got != "app.bash vars.env " {
			t.Errorf("%v: COMPREPLY = %q, want the files", argv, got)
		}
	}
}
