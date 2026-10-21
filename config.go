package cli

import (
	"io"
	"os"
)

// Config holds settings for a whole command tree. Set it on the root
// command; nil fields fall back to the defaults below.
type Config struct {
	// FlagAliases maps a long flag name to its short form, shown together
	// as "-h, --help". Default: help->h, version->v.
	FlagAliases map[string]string
	// HiddenFlags are left out of help and completion. Default: h, v.
	HiddenFlags map[string]bool
	// PathFlags are flags whose value completes as a filesystem path.
	PathFlags map[string]bool
	// Out receives help output. Default: os.Stdout.
	Out io.Writer
	// Err receives diagnostics. Default: os.Stderr.
	Err io.Writer
}

// EffectiveConfig returns the root command's Config with defaults filled in.
func (cmd *Command) EffectiveConfig() *Config {
	var c Config
	if root := cmd.root().Config; root != nil {
		c = *root
	}
	if c.FlagAliases == nil {
		c.FlagAliases = map[string]string{"help": "h", "version": "v"}
	}
	if c.HiddenFlags == nil {
		c.HiddenFlags = map[string]bool{"h": true, "v": true}
	}
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
	return &c
}
