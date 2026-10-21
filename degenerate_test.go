package cli

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func degenerateTrees() map[string]*Command {
	noop := func([]string) error { return nil }
	boolOnly := flag.NewFlagSet("go", flag.ContinueOnError)
	boolOnly.Bool("quiet", false, "Less output")
	return map[string]*Command{
		"root only":       {Name: "app", Run: noop},
		"flagless leaves": {Name: "app", SubCommands: []*Command{{Name: "go", Run: noop}}},
		"bool flags only": {Name: "app", SubCommands: []*Command{{Name: "go", Flags: boolOnly, Run: noop}}},
	}
}

func TestReviewADegenerateTreesGiveWorkingScripts(t *testing.T) {
	for name, tree := range degenerateTrees() {
		t.Run(name, func(t *testing.T) {
			zsh, bash := scripts(t, tree)
			if strings.Contains(bash, "    ) ") {
				t.Errorf("bash has an empty case pattern:\n%s", bash)
			}
			if !strings.Contains(zsh, "\n_app() {") {
				t.Errorf("zsh registers _app with compdef but never defines it:\n%s", zsh)
			}
			for shell, src := range map[string]string{"zsh": zsh, "bash": bash} {
				t.Run(shell, func(t *testing.T) { checkSyntax(t, shell, src) })
			}
		})
	}
}

func shellPath(t *testing.T, shell string) string {
	t.Helper()
	bin, err := exec.LookPath(shell)
	if err == nil {
		return bin
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("%s is required on CI for the syntax check", shell)
	}
	t.Skipf("%s not installed", shell)
	return ""
}

func checkSyntax(t *testing.T, shell, src string) {
	t.Helper()
	bin := shellPath(t, shell)
	path := filepath.Join(t.TempDir(), "script."+shell)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "-n", path).CombinedOutput(); err != nil {
		t.Errorf("%s -n: %v\n%s\n%s", shell, err, out, src)
	}
}
