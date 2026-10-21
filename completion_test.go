package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestBashCompgenPayloadsAreSafe guards against shell-meta characters
// (`$`, backtick, backslash) leaking into compgen -W word lists. The
// generator uses Go's %q which produces double-quoted strings; bash
// expands $ and backtick inside double quotes, so any name containing
// those would either mangle or execute at tab time.
func TestBashCompgenPayloadsAreSafe(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteBashCompletion(&buf, fixtureTree()); err != nil {
		t.Fatal(err)
	}

	for i, line := range strings.Split(buf.String(), "\n") {
		idx := strings.Index(line, `compgen -W "`)
		if idx < 0 {
			continue
		}
		start := idx + len(`compgen -W "`)
		end := strings.Index(line[start:], `"`)
		if end < 0 {
			continue
		}
		payload := line[start : start+end]
		// _hulak_envs uses $(...) substitution by design; not a literal payload.
		if strings.Contains(payload, "_hulak_envs") {
			continue
		}
		for _, bad := range []string{"$", "`", "\\"} {
			if strings.Contains(payload, bad) {
				t.Errorf("line %d: compgen -W payload contains %q (shell-meta): %s", i+1, bad, payload)
			}
		}
	}
}

// TestBashChainWalker shells out to bash to verify chain resolution. The
// walker must skip value-taking flags and stop at non-subcommand positionals.
func TestBashChainWalker(t *testing.T) {
	bashBin, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}

	var script bytes.Buffer
	if err := WriteBashCompletion(&script, fixtureTree()); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "hulak.bash")
	if err := os.WriteFile(scriptPath, script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	// One-shot harness: source the script, replay the chain walker against
	// COMP_WORDS, print the resolved chain. Disabling extglob first
	// confirms the yaml filter doesn't silently rely on it.
	harness := `
shopt -u extglob 2>/dev/null
source "$1"
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
chain=hulak; i=1
while (( i < COMP_CWORD )); do
  w="${COMP_WORDS[i]}"
  if [[ -z $w ]]; then ((i++)); continue; fi
  if [[ $w == -* ]]; then
    _hulak_takes_value "$w" && ((i++))
    ((i++)); continue
  fi
  if _hulak_is_path "$chain:$w"; then chain+=:$w; ((i++)); else break; fi
done
printf '%s' "$chain"
`

	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"flag-with-value", []string{"hulak", "run", "--env", "prod", ""}, "hulak:run"},
		{"positional-then-flag", []string{"hulak", "run", "file.yaml", "--"}, "hulak:run"},
		{"secrets-alias", []string{"hulak", "env", "keys", "get", ""}, "hulak:env:keys:get"},
		{"nested-alias", []string{"hulak", "init", "classic", ""}, "hulak:init:classic"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-c", harness, "_", scriptPath}, tc.argv...)
			out, err := exec.Command(bashBin, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("bash failed: %v\n%s", err, out)
			}
			if got := string(out); got != tc.want {
				t.Errorf("chain = %q, want %q (argv=%v)", got, tc.want, tc.argv)
			}
		})
	}
}
