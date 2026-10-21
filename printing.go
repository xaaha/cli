package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
)

const (
	colorYellow = "\033[93m"
	colorReset  = "\033[0m"
)

// Example is one usage example shown in a command's help.
type Example struct {
	Command     string
	Description string
}

func printSectionHeader(msg string) {
	fmt.Printf("%s%s%s\n", colorYellow, msg, colorReset)
}

type helpEntry struct {
	Command     string
	Description string
}

func writeHelpEntries(commands []*helpEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	for _, cmd := range commands {
		if _, err := fmt.Fprintf(w, "  %s\t- %s\n", cmd.Command, cmd.Description); err != nil {
			return err
		}
	}
	return w.Flush()
}
