package cli

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestReviewVLoneLongFlagDoesNotBorrowShortCompleter(t *testing.T) {
	root := oneFlagTree(&Config{
		FlagAliases:    map[string]string{"output": "o"},
		FlagCompleters: map[string]string{"o": KindFile},
	}, "output", "Output name")
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'--output[Output name]:value:'") {
		t.Errorf("--output borrowed -o's completer in zsh:\n%s", zsh)
	}
	if strings.Contains(bash, "_app_path_files") {
		t.Errorf("--output borrowed -o's completer in bash:\n%s", bash)
	}
}

func TestReviewVZshActionKeepsBackslashAndColon(t *testing.T) {
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

type failOnce struct{ failed bool }

func (f *failOnce) Write(p []byte) (int, error) {
	if !f.failed {
		f.failed = true
		return 0, errDiskFull
	}
	return len(p), nil
}

func TestReviewVFirstWriteErrorSurvivesLaterSuccess(t *testing.T) {
	if err := WriteZshCompletion(&failOnce{}, fixtureTree()); !errors.Is(err, errDiskFull) {
		t.Errorf("zsh: error = %v, want %v", err, errDiskFull)
	}
	if err := WriteBashCompletion(&failOnce{}, fixtureTree()); !errors.Is(err, errDiskFull) {
		t.Errorf("bash: error = %v, want %v", err, errDiskFull)
	}
}

func TestReviewVMissingShellFailsOnCIAndSkipsLocally(t *testing.T) {
	if os.Getenv("CLI_SHELL_RULE_CHILD") != "" {
		t.Skip("child process")
	}
	run := func(ci bool) (string, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestD7ScriptsAreValidShell$", "-test.v") //nolint:gosec // re-runs this test binary
		env := []string{"PATH=/nonexistent", "CLI_SHELL_RULE_CHILD=1"}
		if ci {
			env = append(env, "CI=true")
		}
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := run(true)
	if err == nil || !strings.Contains(out, "is required on CI") {
		t.Errorf("CI without shells: err=%v\n%s", err, out)
	}
	out, err = run(false)
	if err != nil || !strings.Contains(out, "--- SKIP") {
		t.Errorf("local without shells: err=%v\n%s", err, out)
	}
}
