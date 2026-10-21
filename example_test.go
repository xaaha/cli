package cli_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xaaha/cli"
)

func Example() {
	root := &cli.Command{
		Name:  "hello",
		Short: "Say hello",
		Run: func([]string) error {
			fmt.Println("Hello, world!")
			return nil
		},
	}
	if err := root.Execute(nil); err != nil {
		fmt.Println("error:", err)
	}
	if err := root.Execute([]string{"--help"}); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// Hello, world!
	// Say hello
	//
	// USAGE
	//   hello
}

func ExampleCommand_Execute() {
	greetFlags := flag.NewFlagSet("greet", flag.ContinueOnError)
	name := greetFlags.String("name", "world", "Who to greet")

	root := &cli.Command{
		Name: "app",
		Long: "app does one thing well.",
		SubCommands: []*cli.Command{{
			Name:  "greet",
			Short: "Say hello",
			Flags: greetFlags,
			Run: func([]string) error {
				fmt.Println("hello,", *name)
				return nil
			},
		}},
	}
	if err := root.Execute([]string{"greet", "--name", "Ada"}); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// hello, Ada
}

func ExampleCommand_Execute_flagsAnywhere() {
	newTree := func() *cli.Command {
		setFlags := flag.NewFlagSet("set", flag.ContinueOnError)
		env := setFlags.String("env", "dev", "Environment to write to")
		return &cli.Command{
			Name: "app",
			SubCommands: []*cli.Command{{
				Name:  "set",
				Short: "Set a key",
				Flags: setFlags,
				Args:  []cli.ArgDef{{Name: "KEY", Required: true}},
				Run: func(args []string) error {
					fmt.Printf("set %s in %s\n", args[0], *env)
					return nil
				},
			}},
		}
	}
	for _, args := range [][]string{
		{"set", "API_KEY", "--env", "prod"},
		{"set", "--env", "prod", "API_KEY"},
		{"--env", "prod", "set", "API_KEY"},
	} {
		if err := newTree().Execute(args); err != nil {
			fmt.Println("error:", err)
		}
	}
	// Output:
	// set API_KEY in prod
	// set API_KEY in prod
	// set API_KEY in prod
}

func ExampleCommand_Execute_flagsFirst() {
	sayFlags := flag.NewFlagSet("say", flag.ContinueOnError)
	to := sayFlags.String("to", "everyone", "Who hears it")

	root := &cli.Command{
		Name: "chat",
		SubCommands: []*cli.Command{{
			Name:       "say",
			Short:      "Send a message",
			Flags:      sayFlags,
			FlagsFirst: true,
			Run: func(args []string) error {
				fmt.Printf("to %s: %s\n", *to, strings.Join(args, " "))
				return nil
			},
		}},
	}
	if err := root.Execute([]string{"say", "--to", "ada", "use", "--force", "-v", "carefully"}); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// to ada: use --force -v carefully
}

func ExampleCommand_PrintHelp() {
	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.Int("timeout", 30, "Seconds before giving up")

	root := &cli.Command{
		Name: "app",
		Long: "app runs and lists jobs.",
		SubCommands: []*cli.Command{
			{
				Name:    "run",
				Aliases: []string{"r"},
				Short:   "Run a job",
				Long:    "Run one job file.",
				Flags:   runFlags,
				Args:    []cli.ArgDef{{Name: "file", Required: true, Desc: "Job file"}},
				Examples: []cli.Example{
					{Command: "app run nightly.yaml", Description: "Run the nightly job"},
				},
				Run: func([]string) error { return nil },
			},
			{Name: "list", Aliases: []string{"ls"}, Short: "List jobs", Run: func([]string) error { return nil }},
			{Name: "debug", Short: "Internal tools", Hidden: true, Run: func([]string) error { return nil }},
		},
	}
	if err := root.Execute([]string{"--help"}); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println("----")
	if err := root.Execute([]string{"r", "-h"}); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// app runs and lists jobs.
	//
	// COMMANDS
	//   list (ls)    - List jobs
	//   run (r)      - Run a job
	//
	// LEARN MORE
	//   Use `app <command> --help` for more information about a command.
	// ----
	// Run one job file.
	//
	// USAGE
	//   app run [flags] <file>
	//
	// FLAGS
	//   -timeout int
	//     	Seconds before giving up (default "30")
	//
	// ARGUMENTS
	//   <file>    - Job file (required)
	//
	// EXAMPLES
	//   $ app run nightly.yaml
	//     Run the nightly job
}

func ExampleConfig() {
	deployFlags := flag.NewFlagSet("deploy", flag.ContinueOnError)
	deployFlags.String("out", "", "Write the report to this file")
	deployFlags.String("o", "", "Write the report to this file")
	deployFlags.String("env", "dev", "Target environment")
	deployFlags.Bool("trace", false, "Internal tracing")

	root := &cli.Command{
		Name: "app",
		Config: &cli.Config{
			FlagAliases:    map[string]string{"out": "o"},
			HiddenFlags:    map[string]bool{"trace": true},
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
		SubCommands: []*cli.Command{{
			Name:  "deploy",
			Short: "Deploy the app",
			Flags: deployFlags,
			Run:   func([]string) error { return nil },
		}},
	}
	if err := root.Execute([]string{"deploy", "--help"}); err != nil {
		fmt.Println("error:", err)
	}
	var zsh bytes.Buffer
	if err := cli.WriteZshCompletion(&zsh, root); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println("----")
	for line := range strings.Lines(zsh.String()) {
		if strings.Contains(line, "--env") || strings.Contains(line, "--out") {
			fmt.Println(strings.TrimSuffix(strings.TrimSpace(line), " \\"))
		}
	}
	// Output:
	// Deploy the app
	//
	// USAGE
	//   app deploy [flags]
	//
	// FLAGS
	//   -env string
	//     	Target environment (default "dev")
	//   -o, --out string
	//     	Write the report to this file
	// ----
	// '--env[Target environment]:environment:(dev staging prod)'
	// '(--out -o)'{--out,-o}'[Write the report to this file]:path:_files'
}

func completionTree() *cli.Command {
	root := &cli.Command{Name: "app"}
	buildFlags := flag.NewFlagSet("build", flag.ContinueOnError)
	buildFlags.String("out", "", "Output directory")
	root.SubCommands = []*cli.Command{
		{
			Name:  "build",
			Short: "Build a file",
			Flags: buildFlags,
			Args:  []cli.ArgDef{{Name: "file", Kind: cli.KindFile}},
			Run:   func([]string) error { return nil },
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
	return root
}

func ExampleWriteZshCompletion() {
	root := completionTree()
	if err := root.Execute([]string{"completion", "zsh"}); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// #compdef app
	// # Generated code. Do not edit by hand.
	//
	// _app() {
	//   local state ret=1
	//   _arguments -C \
	//     '1: :_app_subs' \
	//     '*::arg:->args' && ret=0
	//   [[ $state == args ]] && case $words[1] in
	//     build) _app_build && ret=0 ;;
	//     completion) _app_completion && ret=0 ;;
	//   esac
	//   return ret
	// }
	//
	// _app_subs() {
	//   local -a subs=(
	//     'build:Build a file'
	//     'completion:Print a shell completion script'
	//   )
	//   _describe -t commands 'app subcommand' subs
	// }
	//
	// _app_build() {
	//   _arguments \
	//     '--out[Output directory]:value:' \
	//     '*:file:_files'
	// }
	//
	// _app_completion() {
	//   _arguments \
	//     '1: :_app_completion_subs'
	// }
	//
	// _app_completion_subs() {
	//   local -a subs=(
	//     'zsh:Print the zsh script'
	//     'bash:Print the bash script'
	//   )
	//   _describe -t commands 'completion subcommand' subs
	// }
	//
	// compdef _app app
}

func ExampleWriteBashCompletion() {
	var script bytes.Buffer
	if err := cli.WriteBashCompletion(&script, completionTree()); err != nil {
		fmt.Println("error:", err)
	}
	lines := strings.Split(strings.TrimSpace(script.String()), "\n")
	fmt.Println(lines[0])
	fmt.Println(lines[len(lines)-1])
	// Output:
	// # bash completion for app
	// complete -F _app app
}
