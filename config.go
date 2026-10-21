package cli

import (
	"flag"
	"io"
	"maps"
	"os"
)

// Config holds settings for a whole command tree. Set it on the root
// command. Nil writers use the defaults below; map entries you set win.
type Config struct {
	// FlagAliases maps a long flag name to its short form, shown together
	// as "-h, --help". Starts with help->h and version->v. The help entry
	// must be empty or a plain flag name, or Execute and the writers fail.
	FlagAliases map[string]string
	// HiddenFlags are left out of help and completion; an entry set to false
	// shows a flag. Unless listed, the help flags added when the tree is
	// linked are hidden, and so is the short help or version flag on a
	// command with its long pair.
	HiddenFlags map[string]bool
	// FlagCompleters maps a flag name to the completion kind of its value:
	// KindFile, KindDir, or a key of Completers. A flag with no entry uses
	// its FlagAliases partner's entry when its FlagSet defines both halves.
	FlagCompleters map[string]string
	// Completers registers app-specific completion kinds by name. A kind the
	// tree uses needs both Zsh and Bash set, or the completion writers fail.
	Completers map[string]Completer
	// Out receives help output. Default: os.Stdout.
	Out io.Writer
	// Err receives diagnostics. Default: os.Stderr.
	Err io.Writer
	// Color controls ANSI colour in help. Default: ColorAuto.
	Color ColorMode
}

// ColorMode selects when help output uses ANSI colour.
type ColorMode int

const (
	// ColorAuto colours only when Out is a terminal and NO_COLOR is unset or empty.
	ColorAuto ColorMode = iota
	// ColorAlways colours regardless of the writer.
	ColorAlways
	// ColorNever never colours.
	ColorNever
)

func (c *Config) useColor() bool {
	return colorEnabled(c.Color, func() bool { return isTerminal(c.Out) })
}

func colorEnabled(mode ColorMode, terminal func() bool) bool {
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	}
	return os.Getenv("NO_COLOR") == "" && terminal()
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func withDefaults[V any](defaults, own map[string]V) map[string]V {
	maps.Copy(defaults, own)
	return defaults
}

func (c *Config) hidden(fs *flag.FlagSet, name string) bool {
	if v, ok := c.HiddenFlags[name]; ok {
		return v
	}
	if injected(fs, name) {
		return true
	}
	for _, long := range []string{"help", "version"} {
		if short := c.FlagAliases[long]; name == short && c.paired(fs, long, short) {
			return true
		}
	}
	return false
}

func (c *Config) paired(fs *flag.FlagSet, long, short string) bool {
	return short != "" && fs.Lookup(long) != nil && fs.Lookup(short) != nil && injected(fs, long) == injected(fs, short)
}

// EffectiveConfig returns a copy of the root command's Config with defaults
// filled in: the help->h and version->v FlagAliases, os.Stdout for Out and
// os.Stderr for Err. A command reaches its root once Execute or a
// completion writer has linked the tree.
func (cmd *Command) EffectiveConfig() *Config {
	var c Config
	if root := cmd.root().Config; root != nil {
		c = *root
	}
	c.FlagAliases = withDefaults(map[string]string{"help": "h", "version": "v"}, c.FlagAliases)
	c.HiddenFlags = withDefaults(map[string]bool{}, c.HiddenFlags)
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
	return &c
}
