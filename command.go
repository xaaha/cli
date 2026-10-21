// Package cli builds command-line tools from a tree of Commands on top of the
// standard flag package: dispatch, help, and zsh and bash completion.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Command represents a CLI command with optional subcommands and flags.
type Command struct {
	Name        string                    // primary name
	Aliases     []string                  // alternative names
	Short       string                    // one-line description for parent's help listing
	Long        string                    // detailed help shown with --help
	Hidden      bool                      // omit from help listings (still callable)
	Examples    []Example                 // usage examples shown under EXAMPLES
	Flags       *flag.FlagSet             // scoped flags for this command
	Args        []ArgDef                  // positional arguments, for help and completion
	SubCommands []*Command                // nested subcommands
	Run         func(args []string) error // handler; nil means subcommand-only
	Config      *Config                   // tree-wide settings; read from the root command only

	parent *Command
}

// ArgDef describes a positional argument for help output and completion.
// Kind selects the value completer: KindFile, KindDir, a key of
// Config.Completers, or "" for none.
type ArgDef struct {
	Name     string
	Required bool
	Desc     string
	Kind     string
}

// Execute dispatches args to the correct subcommand or runs this command.
// It first links the tree, adding --help and its short alias to every FlagSet
// that does not already define them.
// Flag values persist after a call, so build a fresh tree for each run.
func (cmd *Command) Execute(args []string) error {
	if cmd.parent == nil {
		if err := cmd.link(); err != nil {
			return err
		}
	}
	return cmd.execute(args)
}

func (cmd *Command) link() error {
	if err := cmd.addHelpFlags(); err != nil {
		return err
	}
	for _, sub := range cmd.SubCommands {
		sub.parent = cmd
		if err := sub.link(); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *Command) addHelpFlags() error {
	short := cmd.EffectiveConfig().FlagAliases["help"]
	if short != "" && !shellSafeName.MatchString(short) {
		return fmt.Errorf("FlagAliases[\"help\"] = %q is not a valid flag name", short)
	}
	if cmd.Flags == nil {
		return nil
	}
	for _, name := range []string{"help", short} {
		if name != "" && cmd.Flags.Lookup(name) == nil {
			cmd.Flags.Var(&helpFlag{}, name, "Show help for this command")
		}
	}
	return nil
}

func (cmd *Command) execute(args []string) error {
	// No args: show help if subcommand-only, or run with empty args
	if len(args) == 0 {
		if cmd.Run == nil {
			cmd.PrintHelp()
			return nil
		}
		return cmd.Run(args)
	}

	// Check for help request as first arg
	if cmd.isHelpArg(args[0]) {
		cmd.PrintHelp()
		return nil
	}

	// Try to match a subcommand. Scan past leading flags so verb-after-flag
	// (`app keys --env prod list`) dispatches the same as verb-first
	// (`app keys list --env prod`). This mirrors the iterative flag-anywhere
	// parsing further down — order of flags vs verb is not significant.
	if len(cmd.SubCommands) > 0 {
		if idx, firstNonFlag := cmd.FindSubcommandIndex(args); idx >= 0 {
			sub := cmd.FindSub(args[idx])
			rest := append(append([]string(nil), args[:idx]...), args[idx+1:]...)
			return sub.execute(rest)
		} else if firstNonFlag >= 0 {
			// First non-flag token is not a subcommand — that's a typo, not
			// a flag-handler invocation. Show help and exit non-zero.
			fmt.Fprintln(cmd.EffectiveConfig().Err)
			cmd.PrintHelp()
			return fmt.Errorf("unknown command %q for %s", args[firstNonFlag], cmd.path())
		}
	}

	// No subcommand matched — parse flags and run
	if cmd.Flags != nil {
		helpFlags := cmd.injectedHelpFlags()
		for _, h := range helpFlags {
			h.on = false
		}

		// Suppress Go's default usage dump so we can show our own errors
		cmd.Flags.Usage = func() {}
		cmd.Flags.SetOutput(io.Discard)

		// Iterative parse: stdlib flag.Parse stops at the first non-flag argument,
		// so `app set FOO bar --env prod` would leave '--env prod' unparsed.
		// Loop, peeling off one positional per iteration, until all flags (in any position) are consumed.
		// After this, args holds only positionals.
		var positionals []string
		remaining := args
		for {
			if err := cmd.Flags.Parse(remaining); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					err = errors.New("flag provided but not defined: -h")
				}
				return fmt.Errorf("%s\nSee '%s --help' for usage", err, cmd.path())
			}
			if slices.ContainsFunc(helpFlags, func(h *helpFlag) bool { return h.on }) {
				cmd.PrintHelp()
				return nil
			}
			if cmd.Flags.NArg() == 0 {
				break
			}
			if endsFlags(cmd.Flags, remaining) {
				positionals = append(positionals, cmd.Flags.Args()...)
				break
			}
			positionals = append(positionals, cmd.Flags.Arg(0))
			remaining = cmd.Flags.Args()[1:]
		}
		args = positionals
	}

	if cmd.Run == nil {
		cmd.PrintHelp()
		return nil
	}

	return cmd.Run(args)
}

func (cmd *Command) root() *Command {
	r := cmd
	for r.parent != nil {
		r = r.parent
	}
	return r
}

func (cmd *Command) path() string {
	if cmd.parent == nil {
		return cmd.Name
	}
	return cmd.parent.path() + " " + cmd.Name
}

// endsFlags reports whether flag.Parse stops args at a "--" rather than at a positional.
func endsFlags(fs *flag.FlagSet, args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return true
		}
		if len(a) < 2 || a[0] != '-' {
			return false
		}
		name := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		if strings.Contains(name, "=") {
			continue
		}
		if f := fs.Lookup(name); f != nil {
			if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
				continue
			}
		}
		i++
	}
	return false
}

// FindSub returns the subcommand matching name by Name or Aliases, or nil.
func (cmd *Command) FindSub(name string) *Command {
	for _, sub := range cmd.SubCommands {
		if sub.Name == name || slices.Contains(sub.Aliases, name) {
			return sub
		}
	}
	return nil
}

// VisibleSubs returns subcommands that aren't hidden. Help and completion
// skip hidden commands through this one helper.
func (cmd *Command) VisibleSubs() []*Command {
	out := make([]*Command, 0, len(cmd.SubCommands))
	for _, sub := range cmd.SubCommands {
		if !sub.Hidden {
			out = append(out, sub)
		}
	}
	return out
}

// FlagPairings returns a short->long map for flags where both forms are
// registered on fs. Help and completion use it to show `-o, --out` on one line.
func (c *Config) FlagPairings(fs *flag.FlagSet) map[string]string {
	out := make(map[string]string)
	for long, short := range c.FlagAliases {
		if c.paired(fs, long, short) {
			out[short] = long
		}
	}
	return out
}

// VisitVisibleFlags calls fn for each visible flag on fs — that is, every
// flag except hidden ones and long-form aliases whose short partner is also
// registered (the short form pulls the long form in via FlagPairings).
// Same iteration order as flag.FlagSet.VisitAll (alphabetical by name).
func (c *Config) VisitVisibleFlags(fs *flag.FlagSet, fn func(*flag.Flag)) {
	fs.VisitAll(func(f *flag.Flag) {
		if c.hidden(fs, f.Name) || c.paired(fs, f.Name, c.FlagAliases[f.Name]) {
			return
		}
		fn(f)
	})
}

// FindSubcommandIndex scans args for the first non-flag token, skipping
// flags and (where determinable from cmd.Flags) their values. Returns:
//
//   - matchIdx ≥ 0  : args[matchIdx] resolves to a subcommand of cmd.
//   - matchIdx == -1, firstNonFlag ≥ 0 : a positional was found but it
//     isn't a subcommand (typo).
//   - both -1       : args contains only flags / is empty.
//
// Flag-value pairing rules:
//   - `--name=value` inlines the value — the next arg is not consumed.
//   - Bool flags (per the flag.Value IsBoolFlag contract) take no value.
//   - Anything else is assumed to consume the next arg as its value.
//
// This is the dispatch-level analogue of the iterative flag-anywhere parsing
// in Execute: ordering between flags and the verb is not significant.
func (cmd *Command) FindSubcommandIndex(args []string) (matchIdx, firstNonFlag int) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "" {
			continue
		}
		if a[0] != '-' {
			if cmd.FindSub(a) != nil {
				return i, i
			}
			return -1, i
		}
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			// --foo=bar inlines the value; the next arg is not consumed.
			continue
		}
		if cmd.Flags != nil {
			if f := cmd.Flags.Lookup(name); f != nil {
				if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
					continue
				}
			}
		}
		// Unknown flag or known non-bool flag: assume the next arg is its value.
		i++
	}
	return -1, -1
}

// PrintHelp prints the command's help to Config.Out in a style similar to gh CLI.
// A subcommand reads the root's Config, and lists help flags, once Execute or
// a completion writer has linked the tree; before that it acts as its own root.
func (cmd *Command) PrintHelp() {
	cfg := cmd.EffectiveConfig()
	w := cfg.Out
	color := cfg.useColor()
	if cmd.Long != "" {
		fmt.Fprintln(w, cmd.Long)
		fmt.Fprintln(w)
	}

	if subs := cmd.VisibleSubs(); len(subs) > 0 {
		printSectionHeader(w, color, "COMMANDS")
		var entries []*helpEntry
		for _, sub := range subs {
			name := sub.Name
			if len(sub.Aliases) > 0 {
				name += " (" + strings.Join(sub.Aliases, ", ") + ")"
			}
			entries = append(entries, &helpEntry{
				Command:     name,
				Description: sub.Short,
			})
		}
		// Alphabetical so users can scan; registration order is meaningless
		// to readers. Sort by primary name (everything before the first
		// space, i.e. before the "(alias)" suffix).
		slices.SortFunc(entries, func(a, b *helpEntry) int {
			return strings.Compare(primaryName(a.Command), primaryName(b.Command))
		})
		_ = writeHelpEntries(w, entries)
		fmt.Fprintln(w)
	}

	if cmd.Flags != nil {
		printFlags(w, cfg, color, cmd.Flags)
	}

	if len(cmd.Args) > 0 {
		printSectionHeader(w, color, "ARGUMENTS")
		var entries []*helpEntry
		for _, a := range cmd.Args {
			name := "<" + a.Name + ">"
			desc := a.Desc
			if a.Required {
				desc += " (required)"
			}
			entries = append(entries, &helpEntry{
				Command:     name,
				Description: desc,
			})
		}
		_ = writeHelpEntries(w, entries)
		fmt.Fprintln(w)
	}

	if len(cmd.Examples) > 0 {
		printSectionHeader(w, color, "EXAMPLES")
		for _, ex := range cmd.Examples {
			fmt.Fprintf(w, "  $ %s\n", ex.Command)
			fmt.Fprintf(w, "    %s\n", ex.Description)
		}
		fmt.Fprintln(w)
	}

	printSectionHeader(w, color, "LEARN MORE")
	fmt.Fprintf(w, "  Use `%s <command> --help` for more information about a command.\n", cmd.root().Name)
}

// printFlags renders the FLAGS section, merging short/long aliases onto one
// line (e.g. "-o, --out string") and skipping hidden flags.
func printFlags(w io.Writer, cfg *Config, color bool, fs *flag.FlagSet) {
	longFor := cfg.FlagPairings(fs)

	hasVisible := false
	cfg.VisitVisibleFlags(fs, func(*flag.Flag) { hasVisible = true })
	if !hasVisible {
		return
	}

	printSectionHeader(w, color, "FLAGS")
	cfg.VisitVisibleFlags(fs, func(f *flag.Flag) {
		// Build flag name: "-o, --out" or just "-debug"
		label := "  -" + f.Name
		if long, ok := longFor[f.Name]; ok {
			label += ", --" + long
		}

		// Show type hint for non-bool flags using the flag's actual type
		if f.DefValue != "false" && f.DefValue != "true" {
			typeName := fmt.Sprintf("%T", f.Value)
			// flag.Value wraps types as *flag.stringValue, *flag.intValue, etc.
			// Extract the underlying type name from the wrapper
			switch {
			case strings.Contains(typeName, "int"):
				label += " int"
			case strings.Contains(typeName, "float"):
				label += " float"
			case strings.Contains(typeName, "duration"):
				label += " duration"
			default:
				label += " string"
			}
		}

		fmt.Fprintln(w, label)
		usage := f.Usage
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != "0s" {
			usage += fmt.Sprintf(" (default %q)", f.DefValue)
		}
		fmt.Fprintf(w, "    \t%s\n", usage)
	})
	fmt.Fprintln(w)
}

// isHelpArg returns true if the argument is a help request.
func (cmd *Command) isHelpArg(arg string) bool {
	if arg == "help" {
		return true
	}
	var name string
	switch short := cmd.EffectiveConfig().FlagAliases["help"]; {
	case arg == "--help":
		name = "help"
	case short != "" && arg == "-"+short:
		name = short
	default:
		return false
	}
	return cmd.Flags == nil || injected(cmd.Flags, name)
}

type helpFlag struct{ on bool }

func (h *helpFlag) String() string { return strconv.FormatBool(h != nil && h.on) }

func (h *helpFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	h.on = v
	return nil
}

func (cmd *Command) injectedHelpFlags() []*helpFlag {
	var out []*helpFlag
	cmd.Flags.VisitAll(func(f *flag.Flag) {
		if h, ok := f.Value.(*helpFlag); ok {
			out = append(out, h)
		}
	})
	return out
}

func (h *helpFlag) IsBoolFlag() bool { return true }

func injected(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	_, ok := f.Value.(*helpFlag)
	return ok
}

// primaryName strips the "(alias, alias)" suffix from a command label so
// sort comparisons see only the canonical name.
func primaryName(label string) string {
	if before, _, ok := strings.Cut(label, " "); ok {
		return before
	}
	return label
}
