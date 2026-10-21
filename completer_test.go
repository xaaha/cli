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

func kapiCompletionTree() *Command {
	sayFlags := flag.NewFlagSet("say", flag.ContinueOnError)
	sayFlags.String("on", "", "Element id")
	sayFlags.String("attach", "", "File to attach")
	noop := func([]string) error { return nil }
	return &Command{
		Name: "kapi",
		Config: &Config{
			FlagCompleters: map[string]string{"on": "element", "attach": "file"},
			Completers: map[string]Completer{
				"element": {
					Label:     "element id",
					Zsh:       "_kapi_elements",
					ZshFuncs:  "_kapi_elements() {\n  compadd -- $(kapi ids 2>/dev/null)\n}",
					Bash:      func(word string) string { return `COMPREPLY=( $(compgen -W "$(_kapi_elements)" -- ` + word + `) )` },
					BashFuncs: "_kapi_elements() {\n  kapi ids 2>/dev/null\n}",
				},
			},
		},
		SubCommands: []*Command{
			{Name: "say", Short: "Reply as the agent", Flags: sayFlags, Args: []ArgDef{{Name: "page", Required: true, Desc: "Page", Kind: "element"}}, Run: noop},
			{Name: "open", Short: "Open a folder", Args: []ArgDef{{Name: "dir", Required: true, Desc: "Folder", Kind: "dir"}}, Run: noop},
		},
	}
}

func scripts(t *testing.T, root *Command) (zsh, bash string) {
	t.Helper()
	var z, b bytes.Buffer
	if err := WriteZshCompletion(&z, root); err != nil {
		t.Fatal(err)
	}
	if err := WriteBashCompletion(&b, root); err != nil {
		t.Fatal(err)
	}
	return z.String(), b.String()
}

func TestD7RegisteredCompleterAppearsInBothShells(t *testing.T) {
	zsh, bash := scripts(t, kapiCompletionTree())
	for _, want := range []string{
		"#compdef kapi",
		"compdef _kapi kapi",
		"_kapi_elements() {\n  compadd",
		"--on[Element id]:element id:_kapi_elements",
		"'*:element id:_kapi_elements'",
		"--attach[File to attach]:path:_files",
		"'*:directory:_files -/'",
	} {
		if !strings.Contains(zsh, want) {
			t.Errorf("zsh missing %q\n%s", want, zsh)
		}
	}
	for _, want := range []string{
		"complete -F _kapi kapi",
		"_kapi_elements() {\n  kapi ids",
		`--on|-on) COMPREPLY=( $(compgen -W "$(_kapi_elements)" -- "$2") ) ;;`,
		`else COMPREPLY=( $(compgen -W "$(_kapi_elements)" -- "$cur") ); fi`,
		`_kapi_path_files "$2"`,
		"kapi:open)\n      _kapi_dir_files \"$cur\"\n      ;;",
	} {
		if !strings.Contains(bash, want) {
			t.Errorf("bash missing %q\n%s", want, bash)
		}
	}
	for name, s := range map[string]string{"zsh": zsh, "bash": bash} {
		if strings.Contains(s, "hulak") {
			t.Errorf("%s script mentions hulak", name)
		}
	}
}

func TestD7UnknownCompleterIsAnError(t *testing.T) {
	root := &Command{Name: "kapi", SubCommands: []*Command{
		{Name: "say", Args: []ArgDef{{Name: "page", Kind: "nope"}}, Run: func([]string) error { return nil }},
	}}
	for name, write := range map[string]func(*bytes.Buffer, *Command) error{
		"zsh":  func(b *bytes.Buffer, r *Command) error { return WriteZshCompletion(b, r) },
		"bash": func(b *bytes.Buffer, r *Command) error { return WriteBashCompletion(b, r) },
	} {
		var b bytes.Buffer
		err := write(&b, root)
		if err == nil || !strings.Contains(err.Error(), `"nope"`) {
			t.Errorf("%s: error = %v, want unknown completer \"nope\"", name, err)
		}
	}
}

func TestD7ScriptsAreValidShell(t *testing.T) {
	for _, tree := range []*Command{fixtureTree(), kapiCompletionTree()} {
		zsh, bash := scripts(t, tree)
		for shell, src := range map[string]string{"zsh": zsh, "bash": bash} {
			bin, err := exec.LookPath(shell)
			if err != nil {
				requireShellOnCI(t, shell)
				continue
			}
			path := filepath.Join(t.TempDir(), tree.Name+"."+shell)
			if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(bin, "-n", path).CombinedOutput(); err != nil {
				t.Errorf("%s -n %s: %v\n%s", shell, tree.Name, err, out)
			}
		}
	}
}
