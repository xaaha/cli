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

func TestReviewBUnsafeNamesAreRejected(t *testing.T) {
	noop := func([]string) error { return nil }
	badFlag := flag.NewFlagSet("go", flag.ContinueOnError)
	badFlag.String("a$b", "", "x")
	cases := map[string]*Command{
		"root name":    {Name: "my app", Run: noop},
		"command name": {Name: "app", SubCommands: []*Command{{Name: "go'", Run: noop}}},
		"alias":        {Name: "app", SubCommands: []*Command{{Name: "go", Aliases: []string{"$(x)"}, Run: noop}}},
		"flag name":    {Name: "app", SubCommands: []*Command{{Name: "go", Flags: badFlag, Run: noop}}},
	}
	for name, tree := range cases {
		t.Run(name, func(t *testing.T) {
			for shell, write := range map[string]func(*bytes.Buffer, *Command) error{
				"zsh":  func(b *bytes.Buffer, r *Command) error { return WriteZshCompletion(b, r) },
				"bash": func(b *bytes.Buffer, r *Command) error { return WriteBashCompletion(b, r) },
			} {
				var b bytes.Buffer
				if err := write(&b, tree); err == nil || !strings.Contains(err.Error(), "cannot be used in a completion script") {
					t.Errorf("%s: error = %v, want a rejected name", shell, err)
				}
			}
		})
	}
}

func TestReviewBHiddenCommandsAreNotValidated(t *testing.T) {
	root := &Command{Name: "app", SubCommands: []*Command{{Name: "go now", Hidden: true, Run: func([]string) error { return nil }}}}
	if _, _, err := tryScripts(root); err != nil {
		t.Fatalf("hidden command should be ignored: %v", err)
	}
}

func TestReviewBLabelIsEscapedForZsh(t *testing.T) {
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
	bin, err := exec.LookPath("zsh")
	if err != nil {
		requireShellOnCI(t, "zsh")
		return
	}
	path := filepath.Join(t.TempDir(), "app.zsh")
	if err := os.WriteFile(path, []byte(zsh), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "-n", path).CombinedOutput(); err != nil {
		t.Errorf("zsh -n: %v\n%s", err, out)
	}
}

func tryScripts(root *Command) (zsh, bash string, err error) {
	var z, b bytes.Buffer
	if err := WriteZshCompletion(&z, root); err != nil {
		return "", "", err
	}
	if err := WriteBashCompletion(&b, root); err != nil {
		return "", "", err
	}
	return z.String(), b.String(), nil
}
