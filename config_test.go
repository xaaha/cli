package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func treeWithAlias(name, long, short string, out *bytes.Buffer) *Command {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.String(long, "", "the "+long+" flag")
	fs.String(short, "", "the "+long+" flag")
	return &Command{
		Name:   name,
		Flags:  fs,
		Run:    func([]string) error { return nil },
		Config: &Config{FlagAliases: map[string]string{long: short}, Out: out},
	}
}

func helpWith(t *testing.T, cfg *Config) string {
	t.Helper()
	root := &Command{Name: "kapi", Long: "Shared pages", SubCommands: []*Command{{Name: "watch", Short: "Stream events"}}, Config: cfg}
	root.PrintHelp()
	switch w := cfg.Out.(type) {
	case *bytes.Buffer:
		return w.String()
	case *os.File:
		b, err := os.ReadFile(w.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	t.Fatalf("unexpected writer %T", cfg.Out)
	return ""
}

func TestDefaultsApplyWithoutAConfig(t *testing.T) {
	fs := flag.NewFlagSet("kapi", flag.ContinueOnError)
	fs.Bool("version", false, "Print the version")
	fs.Bool("v", false, "Print the version")
	fs.String("file-path", "", "A path")
	fs.String("fp", "", "A path")
	root := &Command{Name: "kapi", Flags: fs, Run: func([]string) error { return nil }}
	out := captureStdout(t, root.PrintHelp)
	if strings.Contains(out, "-v") {
		t.Errorf("-v should be hidden by default:\n%s", out)
	}
	if strings.Contains(out, "-fp, --file-path") {
		t.Errorf("hulak's fp alias must not be a default:\n%s", out)
	}
}

func TestDefaultWritersAreStdoutAndStderr(t *testing.T) {
	cfg := (&Command{Name: "app"}).EffectiveConfig()
	if cfg.Out != os.Stdout || cfg.Err != os.Stderr {
		t.Errorf("defaults: Out=%v Err=%v", cfg.Out, cfg.Err)
	}
}

func TestPartialConfigKeepsDefaultHiddenFlags(t *testing.T) {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	for _, name := range []string{"help", "h", "version", "v"} {
		fs.Bool(name, false, "builtin")
	}
	var out bytes.Buffer
	root := &Command{Name: "app", Flags: fs, Run: func([]string) error { return nil }, Config: &Config{Out: &out}}
	root.PrintHelp()
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "  -h") || strings.HasPrefix(line, "  -v") || strings.HasPrefix(line, "  -help") {
			t.Errorf("default hidden flag shown with a partial Config: %q\n%s", line, out.String())
		}
	}
}

func TestSuppliedMapsAddToDefaults(t *testing.T) {
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

func TestAppFlagAliasesWinOverDefaults(t *testing.T) {
	cfg := (&Command{Name: "app", Config: &Config{FlagAliases: map[string]string{"help": "H"}}}).EffectiveConfig()
	if cfg.FlagAliases["help"] != "H" || cfg.FlagAliases["version"] != "v" {
		t.Errorf("FlagAliases = %v, want the app's help->H plus the default version->v", cfg.FlagAliases)
	}
}

func TestConfigIsPerTree(t *testing.T) {
	cases := []struct{ name, long, short, other string }{
		{"one", "extra", "x", "-y, --yonder"},
		{"two", "yonder", "y", "-x, --extra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for range 50 {
				var out bytes.Buffer
				if err := treeWithAlias(tc.name, tc.long, tc.short, &out).Execute([]string{"--help"}); err != nil {
					t.Fatal(err)
				}
				want := "-" + tc.short + ", --" + tc.long
				if !strings.Contains(out.String(), want) {
					t.Fatalf("help missing %q:\n%s", want, out.String())
				}
				if strings.Contains(out.String(), tc.other) {
					t.Fatalf("help shows the other tree's alias %q:\n%s", tc.other, out.String())
				}
			}
		})
	}
}

func TestHelpGoesToTheConfiguredWriter(t *testing.T) {
	var out bytes.Buffer
	root := &Command{Name: "kapi", Long: "Shared pages", Config: &Config{Out: &out}}
	stdout := captureStdout(t, func() {
		if err := root.Execute([]string{"--help"}); err != nil {
			t.Fatal(err)
		}
	})
	if stdout != "" {
		t.Errorf("help leaked to stdout: %q", stdout)
	}
	if !strings.Contains(out.String(), "Shared pages") {
		t.Errorf("configured writer missing help: %q", out.String())
	}
}

func TestEveryHelpSectionUsesConfigOut(t *testing.T) {
	cases := map[string][]string{
		"help_root":     {"--help"},
		"help_run":      {"run", "--help"},
		"help_secrets":  {"env", "--help"},
		"help_keys_set": {"secrets", "keys", "set", "-h"},
	}
	for golden, args := range cases {
		t.Run(golden, func(t *testing.T) {
			var out bytes.Buffer
			tree := fixtureTree()
			tree.Config.Out = &out
			stdout := captureStdout(t, func() {
				if err := tree.Execute(args); err != nil {
					t.Fatal(err)
				}
			})
			if stdout != "" {
				t.Errorf("help leaked to stdout: %q", stdout)
			}
			want, err := os.ReadFile(filepath.Join("testdata", golden+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) {
				t.Errorf("Config.Out differs from %s.golden\n%s", golden, out.String())
			}
		})
	}
}

func TestUnknownCommandWritesToConfigErr(t *testing.T) {
	var out, errw bytes.Buffer
	tree := fixtureTree()
	tree.Config.Out, tree.Config.Err = &out, &errw
	if err := tree.Execute([]string{"bogus"}); err == nil {
		t.Fatal("want an error for an unknown command")
	}
	if errw.String() != "\n" {
		t.Errorf("Config.Err = %q, want the blank separator line", errw.String())
	}
}

func TestColorOnlyOnATerminal(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "help.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	cases := []struct {
		name      string
		cfg       *Config
		wantColor bool
	}{
		{"auto to a buffer", &Config{Out: &bytes.Buffer{}}, false},
		{"auto to a regular file", &Config{Out: file}, false},
		{"forced on", &Config{Out: &bytes.Buffer{}, Color: ColorAlways}, true},
		{"forced off", &Config{Out: &bytes.Buffer{}, Color: ColorNever}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := helpWith(t, tc.cfg)
			if got := strings.Contains(out, "\x1b["); got != tc.wantColor {
				t.Errorf("colour = %v, want %v:\n%q", got, tc.wantColor, out)
			}
		})
	}
}

func TestNoColorEnvBeatsATerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorEnabled(ColorAuto, func() bool { return true }) {
		t.Error("NO_COLOR must disable colour on a terminal")
	}
	t.Setenv("NO_COLOR", "")
	if !colorEnabled(ColorAuto, func() bool { return true }) {
		t.Error("a terminal without NO_COLOR should get colour")
	}
	if !colorEnabled(ColorAlways, func() bool { return false }) {
		t.Error("ColorAlways must colour non-terminals")
	}
}

func TestAutoColorChecksTheConfiguredWriter(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	t.Setenv("NO_COLOR", "")
	if !(&Config{Out: devNull}).useColor() {
		t.Error("a character device as Out should get colour in auto mode")
	}
	if (&Config{Out: &bytes.Buffer{}}).useColor() {
		t.Error("a buffer as Out must not get colour in auto mode")
	}
}
