package cli

import "flag"

func hulakConfig() *Config {
	return &Config{
		FlagAliases: map[string]string{
			"file-path":   "fp",
			"file":        "f",
			"environment": "env",
			"version":     "v",
			"help":        "h",
			"quiet":       "q",
			"sequential":  "seq",
			"type":        "t",
			"out":         "o",
		},
		HiddenFlags: map[string]bool{"h": true, "v": true},
		Color:       ColorAlways,
		FlagCompleters: map[string]string{
			"out": KindFile, "o": KindFile,
			"file-path": KindFile, "fp": KindFile,
			"file": KindFile, "f": KindFile,
			"ssh-identity": KindFile,
			"dir":          KindFile,
			"dirseq":       KindFile,
			"env":          "env", "environment": "env",
		},
		Completers: map[string]Completer{
			"env": {
				Label: "env",
				Zsh:   "_hulak_envs",
				Bash:  func(word string) string { return `COMPREPLY=( $(compgen -W "$(_hulak_envs)" -- ` + word + `) )` },
				ZshFuncs: `_hulak_envs() {
  [[ -d env ]] || return
  local -a envs=(${(f)"$(cd env && for f in *.env(N); do print -- "${f%.env}"; done)"})
  (( ${#envs} )) && _describe -t envs 'environment' envs
}`,
				BashFuncs: `_hulak_envs() {
  [[ -d env ]] || return 0
  local f
  for f in env/*.env; do [[ -e $f ]] || continue; f=${f##*/}; printf '%s\n' "${f%.env}"; done
}`,
			},
			"yaml": {
				Label: "file or directory",
				Zsh:   `_files -g "*.(yaml|yml|hk.yaml|hk.yml)"`,
				Bash:  func(word string) string { return "_hulak_yaml_files " + word },
				BashFuncs: `_hulak_yaml_files() {
  local cur=$1 f
  COMPREPLY=()
  while IFS= read -r f; do
    case "$f" in *.yaml|*.yml|*.hk.yaml|*.hk.yml) COMPREPLY+=("$f") ;; esac
  done < <(compgen -f -- "$cur")
  while IFS= read -r f; do [[ -n $f ]] && COMPREPLY+=("$f"); done < <(compgen -d -- "$cur")
  compopt -o filenames 2>/dev/null
}`,
			},
		},
	}
}

func fixtureTree() *Command {
	rootFlags := flag.NewFlagSet("hulak", flag.ContinueOnError)
	rootFlags.String("env", "global", "Environment file to use")
	rootFlags.String("environment", "global", "Environment file to use")
	rootFlags.String("fp", "", "Exact path of the request file")
	rootFlags.String("file-path", "", "Exact path of the request file")
	rootFlags.Bool("debug", false, "Print full request and response details")
	rootFlags.Bool("h", false, "Show help")
	rootFlags.Bool("help", false, "Show help")

	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.String("env", "global", "Environment file to use")
	runFlags.String("environment", "global", "Environment file to use")
	runFlags.Int("timeout", 60, "Seconds before a request is cancelled")
	runFlags.String("o", "", "Write responses to this path")
	runFlags.String("out", "", "Write responses to this path")

	keysSetFlags := flag.NewFlagSet("set", flag.ContinueOnError)
	keysSetFlags.String("env", "global", "Environment to write to")

	initFlags := flag.NewFlagSet("init", flag.ContinueOnError)
	initFlags.Bool("env", false, "Also create an env folder")

	classicFlags := flag.NewFlagSet("classic", flag.ContinueOnError)
	classicFlags.Bool("force", false, "Overwrite existing files")

	noop := func([]string) error { return nil }

	return &Command{
		Name:   "hulak",
		Long:   "Hulak is a file-based API client.",
		Flags:  rootFlags,
		Config: hulakConfig(),
		SubCommands: []*Command{
			{
				Name:  "run",
				Short: "Run API request files",
				Long:  "Run one request file or every request file in a directory.",
				Flags: runFlags,
				Args:  []ArgDef{{Name: "path", Required: true, Desc: "File or directory", Kind: "yaml"}},
				Examples: []Example{
					{Command: "hulak run api/users.hk.yaml", Description: "Run one file"},
				},
				Run: noop,
			},
			{
				Name:    "secrets",
				Aliases: []string{"env"},
				Short:   "Manage encrypted environment secrets",
				SubCommands: []*Command{
					{
						Name:  "keys",
						Short: "Manage keys in an environment",
						SubCommands: []*Command{
							{
								Name:  "set",
								Short: "Set a key",
								Flags: keysSetFlags,
								Args: []ArgDef{
									{Name: "KEY", Required: true, Desc: "Key name"},
									{Name: "VALUE", Required: true, Desc: "Key value"},
								},
								Run: noop,
							},
							{Name: "get", Short: "Print a key", Args: []ArgDef{{Name: "KEY", Required: true, Desc: "Key name"}}, Run: noop},
						},
					},
					{Name: "list", Short: "List environments", Run: noop},
				},
			},
			{
				Name:  "init",
				Short: "Initialize a project",
				Flags: initFlags,
				Run:   noop,
				SubCommands: []*Command{
					{Name: "classic", Aliases: []string{"plain", "no-vault"}, Short: "Initialize with the plaintext env/ layout", Flags: classicFlags, Run: noop},
				},
			},
			{
				Name:  "completion",
				Short: "Print a shell completion script",
				SubCommands: []*Command{
					{Name: "bash", Short: "Print the bash completion script", Run: noop},
					{Name: "zsh", Short: "Print the zsh completion script", Run: noop},
				},
			},
			{Name: "import", Short: "Import a file", Args: []ArgDef{{Name: "file", Required: true, Desc: "File to import", Kind: "file"}}, Run: noop},
			{Name: "gendocs", Short: "Generate docs", Hidden: true, Run: noop},
			{Name: "version", Short: "Print the version", Run: noop},
		},
	}
}
