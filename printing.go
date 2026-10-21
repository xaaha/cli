package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
)

const (
	Yellow     = "\033[93m"
	ColorReset = "\033[0m"
)

// PrintSectionHeader prints a yellow section header to stdout. Use for help
// text headings ("COMMANDS", "FLAGS", etc.) — not for warnings. Stays on
// stdout because help output is the program's actual output, not diagnostics.
func PrintSectionHeader(msg string) {
	fmt.Printf("%s%s%s\n", Yellow, msg, ColorReset)
}

// CommandHelp holds a command and its description
type CommandHelp struct {
	Command     string
	Description string
}

// CommandHelp holds a command and its description
func WriteCommandHelp(commands []*CommandHelp) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	for _, cmd := range commands {
		if _, err := fmt.Fprintf(w, "  %s\t- %s\n", cmd.Command, cmd.Description); err != nil {
			return err
		}
	}
	return w.Flush()
}
