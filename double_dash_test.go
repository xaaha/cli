package cli

import (
	"flag"
	"io"
	"slices"
	"testing"
)

func TestReviewAHDoubleDashEndsFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "Verbose")
	var got []string
	root := &Command{Name: "app", Config: &Config{Out: io.Discard},
		SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func(args []string) error { got = args; return nil }}}}
	if err := root.Execute([]string{"leaf", "-v", "--", "a", "-x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !*verbose || !slices.Equal(got, []string{"a", "-x", "--help"}) {
		t.Errorf("verbose=%v args=%v", *verbose, got)
	}
}
