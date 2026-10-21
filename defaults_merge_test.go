package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestReviewNSuppliedMapsAddToDefaults(t *testing.T) {
	cases := map[string]struct {
		cfg        *Config
		wantHidden []string
	}{
		"own aliases":      {&Config{FlagAliases: map[string]string{"out": "o"}}, []string{"  -h", "  -help"}},
		"own hidden flags": {&Config{HiddenFlags: map[string]bool{"secret": true}}, []string{"  -h", "  -help", "  -secret"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := tc.cfg
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			fs.String("out", "", "Out path")
			fs.String("o", "", "Out path")
			fs.Bool("secret", false, "Hidden")
			var out bytes.Buffer
			cfg.Out = &out
			root := &Command{Name: "app", Config: cfg, SubCommands: []*Command{{Name: "run", Flags: fs, Run: func([]string) error { return nil }}}}
			if err := root.Execute([]string{"run", "-o", "x", "--help"}); err != nil {
				t.Fatal(err)
			}
			help := out.String()
			if len(cfg.FlagAliases) > 1 || len(cfg.HiddenFlags) > 1 {
				t.Errorf("EffectiveConfig mutated the caller's maps: %v %v", cfg.FlagAliases, cfg.HiddenFlags)
			}
			if cfg.FlagAliases != nil && !strings.Contains(help, "-o, --out string") {
				t.Errorf("own alias missing:\n%s", help)
			}
			for _, line := range strings.Split(help, "\n") {
				for _, hidden := range tc.wantHidden {
					if line == hidden || strings.HasPrefix(line, hidden+" ") || strings.HasPrefix(line, hidden+",") {
						t.Errorf("flag should be hidden: %q\n%s", line, help)
					}
				}
			}
		})
	}
}
