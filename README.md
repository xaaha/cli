# cli

[![Go Reference](https://pkg.go.dev/badge/github.com/xaaha/cli.svg)](https://pkg.go.dev/github.com/xaaha/cli)

A small command framework for Go programs, built on the standard `flag` package.
Extracted from [hulak](https://github.com/xaaha/hulak).

- Nested subcommands with aliases, and help at every level
- Flags in any position: `app set KEY --env prod` and `app --env prod set KEY` both work
- `FlagsFirst` on a command stops reading flags at its first argument, for free text
- Short and long flag pairs shown on one line (`-o, --out`)
- zsh and bash completion generated from the same tree
- No dependencies outside the standard library

## Install

```sh
go get github.com/xaaha/cli@latest
```

The module needs Go 1.24 or newer.

## Hello world

```go
package main

import (
	"fmt"
	"os"

	"github.com/xaaha/cli"
)

func main() {
	root := &cli.Command{
		Name:  "hello",
		Short: "Say hello",
		Run: func([]string) error {
			fmt.Println("Hello, world!")
			return nil
		},
	}
	if err := root.Execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

```console
$ go run .
Hello, world!
$ go run . --help
Say hello

USAGE
  hello
```

## A command with subcommands and flags

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xaaha/cli"
)

func main() {
	greetFlags := flag.NewFlagSet("greet", flag.ContinueOnError)
	name := greetFlags.String("name", "world", "Who to greet")
	loud := greetFlags.Bool("loud", false, "Shout the greeting")

	root := &cli.Command{
		Name: "app",
		Long: "app greets people.",
	}
	root.SubCommands = []*cli.Command{
		{
			Name:    "greet",
			Aliases: []string{"hi"},
			Short:   "Say hello to someone",
			Flags:   greetFlags,
			Examples: []cli.Example{
				{Command: "app greet --name Ada", Description: "Greet Ada"},
			},
			Run: func([]string) error {
				msg := "hello, " + *name
				if *loud {
					msg = strings.ToUpper(msg)
				}
				fmt.Println(msg)
				return nil
			},
		},
		{
			Name:  "completion",
			Short: "Print a shell completion script",
			SubCommands: []*cli.Command{
				{Name: "zsh", Short: "Print the zsh script", Run: func([]string) error { return cli.WriteZshCompletion(os.Stdout, root) }},
				{Name: "bash", Short: "Print the bash script", Run: func([]string) error { return cli.WriteBashCompletion(os.Stdout, root) }},
			},
		},
	}

	if err := root.Execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

```console
$ go run . greet --name Ada
hello, Ada
$ go run . hi --loud --name Ada
HELLO, ADA
$ go run . --help
app greets people.

COMMANDS
  completion    - Print a shell completion script
  greet (hi)    - Say hello to someone

LEARN MORE
  Use `app <command> --help` for more information about a command.
$ go run . greet --help
Say hello to someone

USAGE
  app greet [flags]

FLAGS
  -loud
    	Shout the greeting
  -name string
    	Who to greet (default "world")

EXAMPLES
  $ app greet --name Ada
    Greet Ada
```

A command with a `Run` handles the positional arguments left after its flags
are parsed. A command without one is a group: run on its own, it prints its
help.

The package documentation on [pkg.go.dev](https://pkg.go.dev/github.com/xaaha/cli)
has runnable examples for each section below.

## Flags anywhere

Each command parses its own `Flags` wherever they appear among its arguments,
so these all reach `set` with `--env prod` and the argument `KEY`:

```console
$ app set KEY --env prod
$ app set --env prod KEY
$ app --env prod set KEY
```

Flags typed before a subcommand's name are passed on to that subcommand. To
find the subcommand, the parent assumes such a flag takes a value unless it is
written as `--flag=value` or the parent's own `FlagSet` defines it as a bool.
So a bool flag that only the subcommand defines belongs after the subcommand's
name: `app set --verbose KEY` works, `app --verbose set KEY` does not.

`--` ends flag parsing. Everything after it reaches `Run` as typed, so
`app set -- --not-a-flag` passes `--not-a-flag` as an argument. A lone `-` is
an ordinary argument.

A bad flag returns an error naming the full command path, for example:

```text
flag provided but not defined: -nope
See 'app greet --help' for usage
```

An unknown subcommand prints the parent's help and returns
`unknown command "bogus" for app`.

## FlagsFirst

Some commands take free text that may itself look like flags. Set
`FlagsFirst: true` and the command stops reading flags at its first
positional argument:

```go
{
	Name:       "say",
	Flags:      sayFlags, // defines --to
	FlagsFirst: true,
	Run: func(args []string) error {
		fmt.Printf("to %s: %s\n", *to, strings.Join(args, " "))
		return nil
	},
}
```

```console
$ chat say --to ada use --force -v carefully
to ada: use --force -v carefully
```

Flags before the first positional still work, including `--help`, and an
unknown flag there is still an error. Completion stops offering flags after
the first positional too.

## Aliases

`Aliases` gives a command other names. They work for dispatch and completion,
and help lists them next to the name: `greet (hi)`.

Flag pairs are set tree-wide in `Config.FlagAliases`, mapping a long name to
its short form. Define both flags on the `FlagSet`, usually bound to the same
variable, and help shows them on one line:

```go
var out string
fs.StringVar(&out, "out", "", "Write the report to this file")
fs.StringVar(&out, "o", "", "Write the report to this file")
// with Config{FlagAliases: map[string]string{"out": "o"}}
```

```text
FLAGS
  -o, --out string
    	Write the report to this file
```

A pair only applies on a command that defines both halves. The defaults are
`help` to `h` and `version` to `v`; entries you set are added to them.

## Help

`help`, `--help` or `-h` as the first argument prints help for that command,
and so does `--help` or `-h` anywhere among its flags. Help goes to
`Config.Out` and `Execute` returns nil.

- A command with subcommands shows its `Long` text, a sorted COMMANDS list
  with aliases, its FLAGS, ARGUMENTS and EXAMPLES, and a LEARN MORE line.
- A command without subcommands shows `Long` (or `Short` when `Long` is
  empty), a USAGE line built from its path, `[flags]` when it has visible
  flags and its `Args` (`<name>` when required, `[<name>]` when not), then
  FLAGS, ARGUMENTS and EXAMPLES.
- Each flag shows a type hint (`int`, `uint`, `float`, `duration` or
  `string`; none for bools) and its default unless it is empty, `false`, `0`
  or `0s`.
- `Hidden: true` keeps a command out of help and completion. It still runs.

`Execute` adds `--help` and the short help flag to every `FlagSet` that does
not define them, and hides them from the FLAGS list. If your command defines
its own `--help` or `-h`, that flag is yours: it is parsed and shown like any
other flag. Set `Config.FlagAliases["help"]` to change the short form, or to
`""` for none. `--help=false` runs the command.

## Config

Settings for the whole tree go in `Config` on the root command. Subcommands
read the root's `Config` once `Execute` or a completion writer has linked the
tree.

| Field | What it does |
| --- | --- |
| `FlagAliases` | Long flag name to short form, shown as `-o, --out`. Defaults: `help` to `h`, `version` to `v`. |
| `HiddenFlags` | Flags left out of help and completion. An entry set to `false` shows a flag that would be hidden. |
| `FlagCompleters` | Flag name to the completion kind of its value: `KindFile`, `KindDir` or a key of `Completers`. |
| `Completers` | App-specific completion kinds, by name. |
| `Out` | Where help goes. Default `os.Stdout`. |
| `Err` | Where diagnostics go. Default `os.Stderr`. |
| `Color` | `ColorAuto` (default), `ColorAlways` or `ColorNever`. |

By default the help flags that `Execute` adds are hidden, and so is a short
`-h` or `-v` on a command that also defines its long `--help` or `--version`.
Map entries you set win over the defaults.

## Shell completion

`WriteZshCompletion` and `WriteBashCompletion` write a completion script for
the whole tree. A common pattern is a `completion` command, as in the example
above. The script completes subcommand names, aliases and flags at every
level, and skips hidden commands and flags.

### zsh

Write the script to a directory in your `fpath`, named after the program with
a leading underscore:

```sh
mkdir -p ~/.zsh/completions
app completion zsh > ~/.zsh/completions/_app
```

Then make sure `~/.zshrc` adds that directory before `compinit` runs:

```sh
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit
compinit
```

Or load it on every start, after `compinit`, with
`source <(app completion zsh)` in `~/.zshrc`.

### bash

Load it on every start by adding this line to `~/.bashrc`:

```sh
source <(app completion bash)
```

Or, with the bash-completion package installed, write it where that package
looks for completions:

```sh
mkdir -p ~/.local/share/bash-completion/completions
app completion bash > ~/.local/share/bash-completion/completions/app
```

### Completing values

Set `ArgDef.Kind` on a command's first argument, or a flag's entry in
`Config.FlagCompleters`, to one of:

- `cli.KindFile` for file paths
- `cli.KindDir` for directories
- the name of a `cli.Completer` in `Config.Completers`

```go
Config: &cli.Config{
	FlagCompleters: map[string]string{"out": cli.KindFile, "env": "env"},
	Completers: map[string]cli.Completer{
		"env": {
			Label: "environment",
			Zsh:   "(dev staging prod)",
			Bash: func(word string) string {
				return `COMPREPLY=( $(compgen -W "dev staging prod" -- ` + word + `) )`
			},
		},
	},
},
```

`Zsh` is a zsh `_arguments` action. `Bash` returns a bash command that fills
`COMPREPLY`; `word` is already quoted. `ZshFuncs` and `BashFuncs` hold helper
functions those actions call, written once into each script.

The writers return an error instead of a broken script when a name cannot be
used safely in a script (it must start with a letter or digit and contain only
letters, digits, `_`, `.` and `-`), when a completion kind is unknown or lacks
`Zsh` or `Bash`, or when two commands would need the same zsh function name
(for example `a-b` and `a_b` under one parent, when both have something to
complete). Bash keeps one list of flags
that take a value for the whole tree, so a flag name that is a bool in one
command and takes a value in another is treated as taking a value everywhere.

## Colour

Help section headers are bright yellow when colour is on. With the default
`ColorAuto`, colour is on only when `Config.Out` is a terminal and the
`NO_COLOR` environment variable is unset or empty. `ColorAlways` and
`ColorNever` ignore both.

## Stability

The module follows [semantic versioning](https://semver.org). It is at v0, so
a minor release may still change the API. Every change is listed in
[CHANGELOG.md](CHANGELOG.md).

## Contributing

Issues and pull requests are welcome. Before opening a pull request, run:

```sh
go test -race ./...
go vet ./...
gofmt -l .
golangci-lint run
```

Use the golangci-lint version pinned in `.tool-versions`; `mise install` or
`asdf install` sets it up.

The completion tests run the generated scripts with `zsh` and `bash`. Without
those shells the tests skip locally; on CI (when `CI` is set) they fail.

Help and completion output is checked against golden files in `testdata`.
After an intended change to that output, regenerate them and review the diff:

```sh
go test -run TestGoldenOutput -update .
git diff testdata
```

Tests are grouped by area: `dispatch_test.go`, `flags_test.go`,
`help_test.go`, `config_test.go`, `validation_test.go`, `completion_test.go`,
`completion_zsh_test.go` and `completion_bash_test.go`. Runnable examples live
in `example_test.go`.

## License

[MIT](LICENSE)
