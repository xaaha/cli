package cli

import (
	"strings"
	"testing"
)

func TestD6ExamplesUseExampleType(t *testing.T) {
	cmd := &Command{
		Name:     "kapi",
		Long:     "Shared pages",
		Examples: []Example{{Command: "kapi watch page.html", Description: "Stream events"}},
	}
	out := captureStdout(t, cmd.PrintHelp)
	for _, want := range []string{"EXAMPLES", "  $ kapi watch page.html", "    Stream events"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q\n%s", want, out)
		}
	}
}
