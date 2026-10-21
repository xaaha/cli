package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestReviewAASharedFlagSetStillHonoursHelp(t *testing.T) {
	shared := flag.NewFlagSet("shared", flag.ContinueOnError)
	shared.String("out", "", "Out path")
	ran := ""
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
		{Name: "a", Flags: shared, Run: func([]string) error { ran = "a"; return nil }},
		{Name: "b", Flags: shared, Run: func([]string) error { ran = "b"; return nil }},
	}}
	if err := root.Execute([]string{"b", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if ran != "" || !strings.Contains(out.String(), "LEARN MORE") {
		t.Errorf("ran=%q, help printed=%v", ran, strings.Contains(out.String(), "LEARN MORE"))
	}
}

func TestReviewAAHelpRequestDoesNotCarryOver(t *testing.T) {
	ran := false
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.String("out", "", "Out path")
	var out bytes.Buffer
	root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
		{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
	}}
	if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if err := root.Execute([]string{"run", "--out", "y"}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("the earlier --help still counted on the next Execute")
	}
}

func TestReviewAAParallelTreesKeepSeparateHelpState(t *testing.T) {
	for _, name := range []string{"one", "two"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for range 50 {
				root, out := helpTree(&Config{}, func(*flag.FlagSet) {})
				if err := root.Execute([]string{"run", "--out", "x", "--help"}); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), "LEARN MORE") {
					t.Fatal("help not printed")
				}
			}
		})
	}
}
