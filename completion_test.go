package cli

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type failAfter struct{ n int }

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errDiskFull
	}
	f.n--
	return len(p), nil
}

var errDiskFull = errors.New("disk full")

type failOnce struct {
	failed     bool
	afterwards int
}

func (f *failOnce) Write(p []byte) (int, error) {
	if !f.failed {
		f.failed = true
		return 0, errDiskFull
	}
	f.afterwards++
	return len(p), nil
}

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

// TestCompletionScripts asserts the user-visible surface of both shells in
// one shot: every visible command and alias, hidden commands skipped, env
// flag wired to _hulak_envs, file globbing for yaml-positional commands,
// no stray Go format directives, and no misleading file completion on
// commands whose positional is an opaque key.
func TestCompletionScripts(t *testing.T) {
	type shell struct {
		name string
		gen  func(*bytes.Buffer) error
		want []string
		deny []string
	}
	shells := []shell{
		{
			name: "zsh",
			gen:  func(b *bytes.Buffer) error { return WriteZshCompletion(b, fixtureTree()) },
			want: []string{
				"#compdef hulak",
				"compdef _hulak hulak",
				"_hulak_envs()",
				"run:Run API request",
				"secrets:Manage encrypted environment secrets",
				"env:Manage encrypted environment secrets", // alias of secrets
				"classic:Initialize with the plaintext env/ layout",
				"plain:Initialize with the plaintext env/ layout",    // alias
				"no-vault:Initialize with the plaintext env/ layout", // alias
				"--env,--environment",                                // grouped via flagAliases
				":env:_hulak_envs",                                   // env value completer wired
				`_files -g "*.(yaml|yml|hk.yaml|hk.yml)"`,            // run/gql positional
				// completion subcommand with bash/zsh leaves visible in subs list
				"completion:Print a shell completion script",
				"bash:Print the bash completion script",
				"zsh:Print the zsh completion script",
			},
			deny: []string{
				// gendocs is hidden — must not appear
				"gendocs)", "gendocs|", "_hulak__gendocs",
				// Empty-leaf functions must not be emitted
				"_hulak_version()", "_hulak_help()", "_hulak_secrets_list()",
				// bash/zsh leaves have nothing to complete — no function emitted
				"_hulak_completion_bash()", "_hulak_completion_zsh()",
			},
		},
		{
			name: "bash",
			gen:  func(b *bytes.Buffer) error { return WriteBashCompletion(b, fixtureTree()) },
			want: []string{
				"complete -F _hulak hulak",
				"_hulak_envs()",
				"_hulak_yaml_files()",
				"_hulak_path_files()",
				"_hulak_takes_value()",
				"_hulak_complete_value()",
				"_hulak_is_path()",
				`compgen -W "$(_hulak_envs)"`,
				`_hulak_path_files "$2"`,
				// Value-taking flags the chain walker must skip past
				"--timeout", "--out", "-o",
				// Paths-with-spaces handling
				"while IFS= read -r f",
				"compopt -o filenames",
				// Chain paths the walker checks against
				"hulak:run", "hulak:secrets:keys:set", "hulak:env:keys:get",
				"hulak:run)",
				"hulak:secrets|hulak:env)",
				"hulak:init:classic|hulak:init:plain|hulak:init:no-vault)",
				// Yaml file completion uses portable case pattern, not extglob
				"*.yaml|*.yml|*.hk.yaml|*.hk.yml",
				// completion subcommand and its bash/zsh leaves wired into the chain walker
				"hulak:completion", "hulak:completion:bash", "hulak:completion:zsh",
				`compgen -W "bash zsh"`,
			},
			deny: []string{
				"gendocs)", "gendocs|", "hulak:gendocs",
				// extglob filter must not return — it requires `shopt -s extglob`
				"@(.yaml", "compgen -f -X '!*@",
			},
		},
	}

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := sh.gen(&buf); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			for _, want := range sh.want {
				if !strings.Contains(out, want) {
					t.Errorf("%s script missing %q", sh.name, want)
				}
			}
			for _, deny := range sh.deny {
				if strings.Contains(out, deny) {
					t.Errorf("%s script must not contain %q", sh.name, deny)
				}
			}
			for _, bad := range []string{"%!(", "MISSING"} {
				if strings.Contains(out, bad) {
					t.Errorf("%s script contains malformed format output %q", sh.name, bad)
				}
			}
		})
	}
}

func TestRegisteredCompleterAppearsInBothShells(t *testing.T) {
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

func TestCompletionScriptsAreValidShell(t *testing.T) {
	for _, tree := range []*Command{fixtureTree(), kapiCompletionTree()} {
		zsh, bash := scripts(t, tree)
		for shell, src := range map[string]string{"zsh": zsh, "bash": bash} {
			t.Run(tree.Name+"/"+shell, func(t *testing.T) { checkSyntax(t, shell, src) })
		}
	}
}

func TestDegenerateTreesGiveWorkingScripts(t *testing.T) {
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

func TestMissingShellFailsOnCIAndSkipsLocally(t *testing.T) {
	if os.Getenv("CLI_SHELL_RULE_CHILD") != "" {
		t.Skip("child process")
	}
	run := func(ci bool) (string, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCompletionScriptsAreValidShell$", "-test.v") //nolint:gosec // re-runs this test binary
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

func TestCompletionWriterLinksTheTree(t *testing.T) {
	var out bytes.Buffer
	root, grp, _ := linkTree(&out)
	if err := WriteZshCompletion(io.Discard, root); err != nil {
		t.Fatal(err)
	}
	if grp.EffectiveConfig().Out != &out {
		t.Error("subcommand does not see the root Config after a completion writer ran")
	}
}

func TestBothCompletionWritersLinkGrandchildren(t *testing.T) {
	for shell, write := range map[string]func(io.Writer, *Command) error{"zsh": WriteZshCompletion, "bash": WriteBashCompletion} {
		var out bytes.Buffer
		leaf := &Command{Name: "leaf", Run: func([]string) error { return nil }}
		root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{{Name: "grp", SubCommands: []*Command{leaf}}}}
		if err := write(io.Discard, root); err != nil {
			t.Fatal(err)
		}
		if leaf.EffectiveConfig().Out != &out {
			t.Errorf("%s writer did not link the grandchild to the root Config", shell)
		}
	}
}

func TestCompletionWritersReturnWriteErrors(t *testing.T) {
	for shell, write := range map[string]func(io.Writer, *Command) error{"zsh": WriteZshCompletion, "bash": WriteBashCompletion} {
		for _, okWrites := range []int{0, 5} {
			if err := write(&failAfter{n: okWrites}, fixtureTree()); !errors.Is(err, errDiskFull) {
				t.Errorf("%s after %d writes: error = %v, want %v", shell, okWrites, err, errDiskFull)
			}
		}
	}
}

func TestCompletionWritersStopAtTheFirstWriteError(t *testing.T) {
	for shell, write := range map[string]func(io.Writer, *Command) error{"zsh": WriteZshCompletion, "bash": WriteBashCompletion} {
		w := &failOnce{}
		if err := write(w, fixtureTree()); !errors.Is(err, errDiskFull) {
			t.Errorf("%s: error = %v, want %v", shell, err, errDiskFull)
		}
		if w.afterwards != 0 {
			t.Errorf("%s kept writing %d times after the first failure", shell, w.afterwards)
		}
	}
}

func TestAliasPairSharesItsCompleter(t *testing.T) {
	for _, registered := range []string{"out", "o"} {
		t.Run("registered on "+registered, func(t *testing.T) {
			fs := flag.NewFlagSet("save", flag.ContinueOnError)
			fs.String("out", "", "Out path")
			fs.String("o", "", "Out path")
			root := &Command{
				Name: "app",
				Config: &Config{
					FlagAliases:    map[string]string{"out": "o"},
					FlagCompleters: map[string]string{registered: KindFile},
				},
				SubCommands: []*Command{{Name: "save", Flags: fs, Run: func([]string) error { return nil }}},
			}
			zsh, bash, err := tryScripts(root)
			if err != nil {
				t.Fatal(err)
			}
			if want := `'(--out -o)'{--out,-o}'[Out path]:path:_files'`; !strings.Contains(zsh, want) {
				t.Errorf("zsh missing %s\n%s", want, zsh)
			}
			if want := `--out|-o|-out) _app_path_files "$2" ;;`; !strings.Contains(bash, want) {
				t.Errorf("bash missing %s\n%s", want, bash)
			}
		})
	}
}

func TestAliasCompleterNeedsBothHalvesRegistered(t *testing.T) {
	root := oneFlagTree(&Config{
		FlagAliases:    map[string]string{"env": "e"},
		FlagCompleters: map[string]string{"env": KindDir},
	}, "e", "Exclude pattern")
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, "'-e[Exclude pattern]:value:'") {
		t.Errorf("-e borrowed --env's completer in zsh:\n%s", zsh)
	}
	if strings.Contains(bash, "_app_dir_files") {
		t.Errorf("-e borrowed --env's completer in bash:\n%s", bash)
	}
}

func TestLoneLongFlagDoesNotBorrowTheShortCompleter(t *testing.T) {
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

func TestInjectedHelpFlagsStayOutOfCompletion(t *testing.T) {
	root, _ := helpTree(&Config{FlagAliases: map[string]string{"help": "H"}}, func(fs *flag.FlagSet) { fs.String("h", "", "Host") })
	if err := root.Execute([]string{"run", "--out", "x"}); err != nil {
		t.Fatal(err)
	}
	zsh, bash, err := tryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(zsh, "--help") || strings.Contains(zsh, "-H[") || strings.Contains(bash, "--help") || strings.Contains(bash, " -H") {
		t.Errorf("injected help flags leaked into completion:\nzsh:\n%s\nbash:\n%s", zsh, bash)
	}
}
