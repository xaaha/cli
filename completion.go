package cli

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Built-in completion kinds for ArgDef.Kind and Config.FlagCompleters.
const (
	KindFile = "file"
	KindDir  = "dir"
)

// Completer completes a value of one named kind in both shells.
type Completer struct {
	// Label describes the value in zsh, e.g. "environment".
	Label string
	// Zsh is the zsh action, e.g. `_files -g "*.yaml"` or a function name.
	Zsh string
	// Bash returns a bash command that fills COMPREPLY for the quoted word.
	Bash func(word string) string
	// ZshFuncs and BashFuncs hold helper function definitions the actions call.
	ZshFuncs  string
	BashFuncs string
}

// WriteZshCompletion writes a zsh completion script for the tree under root.
func WriteZshCompletion(w io.Writer, root *Command) error {
	root.link()
	cfg := root.EffectiveConfig()
	if _, err := validateTree(cfg, root); err != nil {
		return err
	}
	name := root.Name
	fn := "_" + shellFn(name)
	p := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	p("#compdef %s", name)
	p("# Generated code. Do not edit by hand.")
	p("")
	emitZshCmd(p, cfg, root, fn)
	if !zshSubHasCompleter(cfg, root) {
		p("%s() {", fn)
		p("  return 1")
		p("}")
		p("")
	}
	for _, kind := range sortedKeys(registered(cfg)) {
		if funcs := cfg.Completers[kind].ZshFuncs; funcs != "" {
			fmt.Fprintln(w, funcs)
			p("")
		}
	}
	p("compdef %s %s", fn, name)
	return nil
}

// WriteBashCompletion writes a bash completion script for the tree under root.
// Bash keeps one tree-wide list of flags that take a value, so a flag name
// that is a bool in one command and takes a value in another is treated as
// taking a value everywhere.
func WriteBashCompletion(w io.Writer, root *Command) error {
	root.link()
	cfg := root.EffectiveConfig()
	used, err := validateTree(cfg, root)
	if err != nil {
		return err
	}
	name := root.Name
	fn := "_" + shellFn(name)
	p := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	ctx := collectBashCtx(cfg, root)

	p("# bash completion for %s", name)
	p("# Generated code. Do not edit by hand.")
	p("")
	if used[KindFile] {
		// Path completion that handles filenames with spaces: read compgen
		// output one line at a time instead of word-splitting on $(...).
		// `compopt -o filenames` asks bash to quote and append slashes correctly.
		emitBashReader(p, fn+"_path_files", "-f")
	}
	if used[KindDir] {
		emitBashReader(p, fn+"_dir_files", "-d")
	}
	for _, kind := range sortedKeys(registered(cfg)) {
		if funcs := cfg.Completers[kind].BashFuncs; funcs != "" {
			fmt.Fprintln(w, funcs)
			p("")
		}
	}
	p("%s_takes_value() {", fn)
	if len(ctx.valueFlags) > 0 {
		p("  case \"$1\" in")
		p("    %s) return 0 ;;", strings.Join(sortedKeys(ctx.valueFlags), "|"))
		p("  esac")
	}
	p("  return 1")
	p("}")
	p("")
	p("%s_complete_value() {", fn)
	p("  case \"$1\" in")
	kinds := make([]string, 0, len(ctx.kindFlags))
	for kind := range ctx.kindFlags {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		p("    %s) %s ;;", strings.Join(sortedKeys(ctx.kindFlags[kind]), "|"), bashComplete(cfg, fn, kind, `"$2"`))
	}
	p("    *) COMPREPLY=() ;;")
	p("  esac")
	p("}")
	p("")
	p("%s_is_path() {", fn)
	p("  case \"$1\" in")
	p("    %s) return 0 ;;", strings.Join(ctx.validPaths, "|"))
	p("  esac")
	p("  return 1")
	p("}")
	p("")
	p("%s() {", fn)
	p("  local cur=\"${COMP_WORDS[COMP_CWORD]}\" prev=\"${COMP_WORDS[COMP_CWORD-1]}\" chain=%s i=1", name)
	p("  if %s_takes_value \"$prev\"; then %s_complete_value \"$prev\" \"$cur\"; return; fi", fn, fn)
	p("  while (( i < COMP_CWORD )); do")
	p("    local w=\"${COMP_WORDS[i]}\"")
	p("    if [[ -z $w ]]; then ((i++)); continue; fi")
	p("    if [[ $w == -* ]]; then")
	p("      %s_takes_value \"$w\" && ((i++))", fn)
	p("      ((i++)); continue")
	p("    fi")
	p("    if %s_is_path \"$chain:$w\"; then chain+=:$w; ((i++)); else break; fi", fn)
	p("  done")
	p("  case \"$chain\" in")
	bashEmitCases(p, cfg, fn, root, []string{name})
	p("    *) COMPREPLY=() ;;")
	p("  esac")
	p("}")
	p("")
	p("complete -F %s %s", fn, name)
	return nil
}

// --- shared helpers ---

func allNames(cmd *Command) []string {
	return append([]string{cmd.Name}, cmd.Aliases...)
}

func dashed(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// argKind returns the completion kind for the first positional, or "" for
// none. Commands with no Args have nothing to complete.
func argKind(cmd *Command) string {
	if len(cmd.Args) == 0 {
		return ""
	}
	return cmd.Args[0].Kind
}

func shellFn(name string) string { return strings.ReplaceAll(name, "-", "_") }

func registered(cfg *Config) map[string]bool {
	out := make(map[string]bool, len(cfg.Completers))
	for kind := range cfg.Completers {
		out[kind] = true
	}
	return out
}

// validateTree returns every completion kind the tree refers to. It fails on
// a kind that is neither built in nor registered, and on a visible name that
// would need quoting inside a shell script.
func validateTree(cfg *Config, root *Command) (map[string]bool, error) {
	used := map[string]bool{}
	check := func(kind, where string) error {
		if kind == "" {
			return nil
		}
		if kind != KindFile && kind != KindDir {
			c, ok := cfg.Completers[kind]
			if !ok {
				return fmt.Errorf("%s uses unknown completer %q", where, kind)
			}
			if c.Zsh == "" || c.Bash == nil {
				return fmt.Errorf("%s: completer %q needs Zsh and Bash", where, kind)
			}
		}
		used[kind] = true
		return nil
	}
	name := func(n, where string) error {
		if !shellSafeName.MatchString(n) {
			return fmt.Errorf("%s: name %q cannot be used in a completion script", where, n)
		}
		return nil
	}
	var walk func(cmd *Command, path string) error
	walk = func(cmd *Command, path string) error {
		for _, n := range allNames(cmd) {
			if err := name(n, path); err != nil {
				return err
			}
		}
		if err := check(argKind(cmd), path); err != nil {
			return err
		}
		if cmd.Flags != nil {
			var err error
			cmd.Flags.VisitAll(func(f *flag.Flag) {
				if err != nil || cfg.HiddenFlags[f.Name] {
					return
				}
				if err = name(f.Name, path+" "+dashed(f.Name)); err == nil && !isBoolFlag(f) {
					err = check(flagKind(cfg, cmd.Flags, f.Name), path+" "+dashed(f.Name))
				}
			})
			if err != nil {
				return err
			}
		}
		for _, sub := range cmd.VisibleSubs() {
			if err := walk(sub, path+" "+sub.Name); err != nil {
				return err
			}
		}
		return nil
	}
	return used, walk(root, root.Name)
}

var shellSafeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// flagKind returns the completion kind for a flag, falling back to the kind
// registered on the other half of its alias pair when fs has both halves.
func flagKind(cfg *Config, fs *flag.FlagSet, name string) string {
	if kind := cfg.FlagCompleters[name]; kind != "" {
		return kind
	}
	for long, short := range cfg.FlagAliases {
		partner := ""
		switch name {
		case long:
			partner = short
		case short:
			partner = long
		}
		if partner != "" && fs.Lookup(partner) != nil {
			return cfg.FlagCompleters[partner]
		}
	}
	return ""
}

func isBoolFlag(f *flag.Flag) bool {
	return f.DefValue == "true" || f.DefValue == "false"
}

// --- zsh ---

// emitZshCmd writes the completer for cmd and recurses into its visible
// subcommands. Function name carries the path (e.g. _app_secrets_set).
// Leaf commands with nothing to complete emit no function and no dispatch.
func emitZshCmd(p func(string, ...any), cfg *Config, cmd *Command, fn string) {
	subs := cmd.VisibleSubs()
	specs := zshFlagSpecs(cfg, cmd)

	if len(subs) == 0 {
		if pos := zshPositionalSpec(cfg, cmd); pos != "" {
			specs = append(specs, pos)
		}
		if len(specs) == 0 {
			return // nothing to complete — skip the function entirely
		}
		emitZshArgs(p, fn, specs)
		return
	}

	var dispatchSubs []*Command
	for _, sub := range subs {
		if zshSubHasCompleter(cfg, sub) {
			dispatchSubs = append(dispatchSubs, sub)
		}
	}

	p("%s() {", fn)
	if len(dispatchSubs) == 0 {
		// No sub needs further dispatch — single-pass _arguments is enough.
		p("  _arguments \\")
		for _, s := range specs {
			p("    %s \\", s)
		}
		p("    '1: :%s_subs'", fn)
		p("}")
		p("")
	} else {
		p("  local state ret=1")
		p("  _arguments -C \\")
		for _, s := range specs {
			p("    %s \\", s)
		}
		p("    '1: :%s_subs' \\", fn)
		p("    '*::arg:->args' && ret=0")
		p("  [[ $state == args ]] && case $words[1] in")
		for _, sub := range dispatchSubs {
			p("    %s) %s_%s && ret=0 ;;", strings.Join(allNames(sub), "|"), fn, shellFn(sub.Name))
		}
		p("  esac")
		p("  return ret")
		p("}")
		p("")
	}

	p("%s_subs() {", fn)
	p("  local -a subs=(")
	for _, sub := range subs {
		for _, name := range allNames(sub) {
			p("    %s", zshDesc(name, sub.Short))
		}
	}
	p("  )")
	p("  _describe -t commands '%s subcommand' subs", cmd.Name)
	p("}")
	p("")

	for _, sub := range subs {
		emitZshCmd(p, cfg, sub, fn+"_"+shellFn(sub.Name))
	}
}

// zshSubHasCompleter reports whether emitZshCmd would emit a function for
// sub. Used to skip dispatch lines that would call into nothing.
func zshSubHasCompleter(cfg *Config, sub *Command) bool {
	if len(sub.VisibleSubs()) > 0 {
		return true
	}
	return len(zshFlagSpecs(cfg, sub)) > 0 || zshPositionalSpec(cfg, sub) != ""
}

func emitZshArgs(p func(string, ...any), fn string, specs []string) {
	p("%s() {", fn)
	p("  _arguments \\")
	for i, s := range specs {
		tail := " \\"
		if i == len(specs)-1 {
			tail = ""
		}
		p("    %s%s", s, tail)
	}
	p("}")
	p("")
}

func zshPositionalSpec(cfg *Config, cmd *Command) string {
	switch kind := argKind(cmd); kind {
	case "":
		return ""
	case KindFile:
		return "'*:file:_files'"
	case KindDir:
		return "'*:directory:_files -/'"
	default:
		c := cfg.Completers[kind]
		return "'*:" + zshEscLabel(c.Label) + ":" + c.Zsh + "'"
	}
}

// zshFlagSpecs returns one `_arguments` spec per visible flag. Aliased
// pairs (per Config.FlagAliases) merge into a single mutex spec like
// `(-o --out){-o,--out}'[desc]:path:_files'`.
func zshFlagSpecs(cfg *Config, cmd *Command) []string {
	if cmd.Flags == nil {
		return nil
	}

	type spec struct {
		name    string
		forms   []string
		usage   string
		isBool  bool
		valKind string
	}
	var specs []spec

	longFor := cfg.FlagPairings(cmd.Flags)
	cfg.VisitVisibleFlags(cmd.Flags, func(f *flag.Flag) {
		s := spec{
			name:    f.Name,
			usage:   f.Usage,
			isBool:  isBoolFlag(f),
			forms:   []string{dashed(f.Name)},
			valKind: flagKind(cfg, cmd.Flags, f.Name),
		}
		if long, ok := longFor[f.Name]; ok {
			s.forms = append(s.forms, dashed(long))
		}
		// Sort by ASCII: -- sorts before -, so long form lists first.
		// Cosmetic only — zsh treats mutex group members as a set.
		sort.Strings(s.forms)
		specs = append(specs, s)
	})
	slices.SortFunc(specs, func(a, b spec) int { return strings.Compare(a.name, b.name) })

	out := make([]string, 0, len(specs))
	for _, s := range specs {
		desc := "[" + zshEscBracket(s.usage) + "]"
		action := zshAction(cfg, s.isBool, s.valKind)
		if len(s.forms) > 1 {
			out = append(out, fmt.Sprintf("'(%s)'{%s}'%s%s'",
				strings.Join(s.forms, " "), strings.Join(s.forms, ","), desc, action))
		} else {
			out = append(out, fmt.Sprintf("'%s%s%s'", s.forms[0], desc, action))
		}
	}
	return out
}

func zshAction(cfg *Config, isBool bool, valKind string) string {
	if isBool {
		return ""
	}
	switch valKind {
	case "":
		return ":value:"
	case KindFile:
		return ":path:_files"
	case KindDir:
		return ":directory:_files -/"
	default:
		c := cfg.Completers[valKind]
		return ":" + zshEscLabel(c.Label) + ":" + c.Zsh
	}
}

func zshDesc(name, short string) string {
	first, _, _ := strings.Cut(short, "\n")
	first = strings.ReplaceAll(first, "'", `'\''`)
	first = strings.ReplaceAll(first, ":", `\:`)
	return fmt.Sprintf("'%s:%s'", name, first)
}

func zshEscLabel(s string) string {
	for _, pair := range [][2]string{{"\\", `\\`}, {"'", `'\''`}, {":", `\:`}} {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}
	return s
}

func zshEscBracket(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	for _, pair := range [][2]string{
		{"\\", `\\`}, {"'", `'\''`}, {"[", `\[`}, {"]", `\]`}, {":", `\:`},
	} {
		first = strings.ReplaceAll(first, pair[0], pair[1])
	}
	return first
}

// --- bash ---

// bashCtx is the data the bash chain walker needs at runtime: every valid
// alias-expanded chain ("app:secrets:set"), and which flags consume a
// value (so the walker skips that value when chaining). Collected once so
// the generated script doesn't re-walk the tree on every tab press.
type bashCtx struct {
	validPaths []string                   // alias-expanded chains
	valueFlags map[string]bool            // dashed names that consume the next word
	kindFlags  map[string]map[string]bool // completion kind -> dashed names whose value it completes
}

func collectBashCtx(cfg *Config, root *Command) bashCtx {
	ctx := bashCtx{
		valueFlags: map[string]bool{},
		kindFlags:  map[string]map[string]bool{},
	}
	addForm := func(set map[string]bool, name string) {
		set["-"+name] = true
		if len(name) > 1 {
			set["--"+name] = true
		}
	}
	var walk func(cmd *Command, patterns []string)
	walk = func(cmd *Command, patterns []string) {
		ctx.validPaths = append(ctx.validPaths, patterns...)
		if cmd.Flags != nil {
			cmd.Flags.VisitAll(func(f *flag.Flag) {
				if cfg.HiddenFlags[f.Name] || isBoolFlag(f) {
					return
				}
				addForm(ctx.valueFlags, f.Name)
				if kind := flagKind(cfg, cmd.Flags, f.Name); kind != "" {
					if ctx.kindFlags[kind] == nil {
						ctx.kindFlags[kind] = map[string]bool{}
					}
					addForm(ctx.kindFlags[kind], f.Name)
				}
			})
		}
		for _, sub := range cmd.VisibleSubs() {
			next := make([]string, 0, len(patterns)*len(allNames(sub)))
			for _, base := range patterns {
				for _, name := range allNames(sub) {
					next = append(next, base+":"+name)
				}
			}
			walk(sub, next)
		}
	}
	walk(root, []string{root.Name})
	sort.Strings(ctx.validPaths)
	return ctx
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func emitBashReader(p func(string, ...any), name, compgenFlag string) {
	p("%s() {", name)
	p("  local cur=$1 f")
	p("  COMPREPLY=()")
	p("  while IFS= read -r f; do [[ -n $f ]] && COMPREPLY+=(\"$f\"); done < <(compgen %s -- \"$cur\")", compgenFlag)
	p("  compopt -o filenames 2>/dev/null")
	p("}")
	p("")
}

func bashComplete(cfg *Config, fn, kind, word string) string {
	switch kind {
	case KindFile:
		return fn + "_path_files " + word
	case KindDir:
		return fn + "_dir_files " + word
	default:
		return cfg.Completers[kind].Bash(word)
	}
}

// bashEmitCases walks the tree emitting one case clause per visible command
// path. patterns is every alias-expanded path leading to cmd; extended as
// we recurse so we never re-resolve aliases.
func bashEmitCases(p func(string, ...any), cfg *Config, fn string, cmd *Command, patterns []string) {
	subs := cmd.VisibleSubs()
	var sugg []string
	for _, sub := range subs {
		sugg = append(sugg, allNames(sub)...)
	}
	if cmd.Flags != nil {
		cmd.Flags.VisitAll(func(f *flag.Flag) {
			if !cfg.HiddenFlags[f.Name] {
				sugg = append(sugg, dashed(f.Name))
			}
		})
	}
	sort.Strings(sugg)

	if kind := argKind(cmd); len(sugg) > 0 || kind != "" {
		p("    %s)", strings.Join(patterns, "|"))
		switch {
		case kind != "" && len(sugg) == 0:
			p("      %s", bashComplete(cfg, fn, kind, `"$cur"`))
		case kind != "":
			p("      if [[ $cur == -* ]]; then COMPREPLY=( $(compgen -W %q -- \"$cur\") )", strings.Join(sugg, " "))
			p("      else %s; fi", bashComplete(cfg, fn, kind, `"$cur"`))
		default:
			p("      COMPREPLY=( $(compgen -W %q -- \"$cur\") )", strings.Join(sugg, " "))
		}
		p("      ;;")
	}

	for _, sub := range subs {
		next := make([]string, 0, len(patterns)*len(allNames(sub)))
		for _, base := range patterns {
			for _, name := range allNames(sub) {
				next = append(next, base+":"+name)
			}
		}
		bashEmitCases(p, cfg, fn, sub, next)
	}
}
