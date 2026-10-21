# cli

A small command framework for Go CLIs, built on the standard `flag` package.
Extracted from [hulak](https://github.com/xaaha/hulak).

- Nested subcommands with aliases, and help at every level
- Flags in any position: `app set KEY --env prod` and `app --env prod set KEY` both work
- Short and long flag pairs shown on one line (`-o, --out`)
- zsh and bash completion generated from the same tree
- No dependencies outside the standard library

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/xaaha/cli"
)

func main() {
	fs := flag.NewFlagSet("greet", flag.ContinueOnError)
	name := fs.String("name", "world", "Who to greet")

	root := &cli.Command{
		Name: "app",
		Long: "app does one thing well.",
		SubCommands: []*cli.Command{{
			Name:     "greet",
			Short:    "Say hello",
			Flags:    fs,
			Examples: []cli.Example{{Command: "app greet --name Ada", Description: "Greet Ada"}},
			Run: func([]string) error {
				fmt.Println("hello,", *name)
				return nil
			},
		}},
	}
	if err := root.Execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

Settings for the whole tree (flag aliases, hidden flags, completion kinds,
output writers, colour) go in `root.Config`. Print completion scripts with
`cli.WriteZshCompletion` and `cli.WriteBashCompletion`.
