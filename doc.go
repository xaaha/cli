// Package cli builds command-line programs from a tree of [Command] values on
// top of the standard flag package. It has no dependencies outside the
// standard library.
//
// A Command has a name, an optional [flag.FlagSet], an optional Run function
// and optional subcommands. Build the tree as a literal and pass the
// program's arguments to [Command.Execute] on the root:
//
//	root := &cli.Command{
//		Name:  "hello",
//		Short: "Say hello",
//		Run: func([]string) error {
//			fmt.Println("Hello, world!")
//			return nil
//		},
//	}
//	if err := root.Execute(os.Args[1:]); err != nil {
//		fmt.Fprintln(os.Stderr, err)
//		os.Exit(1)
//	}
//
// # Dispatch and flags
//
// Execute walks down the tree by the first word that is not a flag, matching
// a subcommand's Name or any of its Aliases. A word that names no subcommand
// is an error naming the full command path. Once it reaches a command,
// Execute parses that command's Flags wherever they appear, so
// "app set KEY --env prod" and "app --env prod set KEY" mean the same, and
// calls Run with the positional arguments that remain. Words after "--" are
// never parsed as flags. Set [Command.FlagsFirst] to stop reading flags at
// the first positional argument, for commands that take free text.
//
// # Help
//
// Execute adds --help and -h to every FlagSet that does not define them and
// hides them from help unless [Config.HiddenFlags] says otherwise. The short
// form comes from [Config.FlagAliases]. "help", --help or -h as the first
// argument, or the help flag among a command's flags, prints that command's
// help to [Config.Out] instead of running it. A command with subcommands
// lists them, sorted by name and with their aliases. A command without them
// shows a USAGE line. Hidden commands are left out of help and completion
// but still run.
//
// # Configuration
//
// Settings for the whole tree live in a [Config] on the root command: flag
// alias pairs shown as "-o, --out", hidden flags, how flag values and
// arguments complete, the output writers and colour.
//
// # Shell completion
//
// [WriteZshCompletion] and [WriteBashCompletion] write completion scripts for
// the same tree. Set [ArgDef.Kind] and [Config.FlagCompleters] to [KindFile],
// [KindDir] or a [Completer] registered in [Config.Completers] to complete
// values.
package cli
