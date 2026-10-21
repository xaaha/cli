package cli

import (
	"io"
	"slices"
	"strings"
	"testing"
)

func TestReviewASDoubleDashStopsSubcommandDispatch(t *testing.T) {
	var rootArgs []string
	statusRan := false
	root := &Command{Name: "app", Config: &Config{Out: io.Discard},
		Run:         func(args []string) error { rootArgs = args; return nil },
		SubCommands: []*Command{{Name: "status", Run: func([]string) error { statusRan = true; return nil }}},
	}
	for _, args := range [][]string{{"--", "status"}, {"--", "x", "status"}} {
		statusRan, rootArgs = false, nil
		if err := root.Execute(args); err != nil {
			t.Fatal(err)
		}
		if statusRan || !slices.Equal(rootArgs, args) {
			t.Errorf("%v: statusRan=%v rootArgs=%v, want the flagless root to get its args unchanged", args, statusRan, rootArgs)
		}
	}
}

func TestReviewASLoneDashIsAPositionalNotAFlag(t *testing.T) {
	ran := false
	root := &Command{Name: "app", Config: &Config{Out: io.Discard, Err: io.Discard},
		SubCommands: []*Command{{Name: "status", Run: func([]string) error { ran = true; return nil }}},
	}
	err := root.Execute([]string{"-", "status"})
	if ran || err == nil || !strings.Contains(err.Error(), `unknown command "-"`) {
		t.Errorf("ran=%v err=%v, want unknown command \"-\"", ran, err)
	}
}
