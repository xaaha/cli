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

func TestBashGeneratedFunctionCompletes(t *testing.T) {
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
		{"a quoted value containing = right before the cursor is not a separator", []string{"hulak", "--env", `"a=b"`, ""}, "--debug --env --environment --file-path --fp --help completion env import init run secrets version ", ""},
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

func TestBashAfterDashesReachesNestedAndAliasedCommands(t *testing.T) {
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

func TestBashAfterDashesCompletesPositionalsAtEveryLevel(t *testing.T) {
	bashBin := shellPath(t, "bash")
	noop := func([]string) error { return nil }
	root := &Command{Name: "app", Args: []ArgDef{{Name: "f", Kind: KindFile}}, Run: noop, SubCommands: []*Command{
		{Name: "par", Args: []ArgDef{{Name: "d", Kind: KindDir}}, Run: noop, SubCommands: []*Command{
			{Name: "kid", Args: []ArgDef{{Name: "f", Kind: KindFile}, {Name: "d", Kind: KindDir}}, Run: noop},
		}},
		{Name: "x)y;echo PWNED", Hidden: true, Args: []ArgDef{{Name: "f", Kind: KindFile}}, Run: noop},
	}}
	var script bytes.Buffer
	if err := WriteBashCompletion(&script, root); err != nil {
		t.Fatal(err)
	}
	checkSyntax(t, "bash", script.String())
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "app.bash")
	if err := os.WriteFile(scriptPath, script.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	harness := `
source "$1"
COMP_WORDS=("${@:2}")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
_app
printf '%s\n' "${COMPREPLY[@]}" | sort | tr '\n' ' '
`
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"app", "--", ""}, "app.bash sub "},
		{[]string{"app", "par", "--", ""}, "sub "},
		{[]string{"app", "par", "kid", "--", ""}, "app.bash sub "},
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
}

func TestBashFlagsFirstStopsCompletingFlagsAfterTheFirstPositional(t *testing.T) {
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

func TestBashFlagsFirstNestedUnderAFlagsFirstParent(t *testing.T) {
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

func TestBashIgnoresZshNameCollisions(t *testing.T) {
	a := flag.NewFlagSet("a-b", flag.ContinueOnError)
	a.String("x", "", "x")
	b := flag.NewFlagSet("a_b", flag.ContinueOnError)
	b.String("y", "", "y")
	root := &Command{Name: "app", SubCommands: []*Command{
		{Name: "a-b", Flags: a, Run: func([]string) error { return nil }},
		{Name: "a_b", Flags: b, Run: func([]string) error { return nil }},
	}}
	if err := WriteBashCompletion(&bytes.Buffer{}, root); err != nil {
		t.Errorf("bash has no per-command functions: %v", err)
	}
}
