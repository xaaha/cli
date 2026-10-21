package cli

import (
	"bytes"
	"flag"
	"io"
	"slices"
	"strings"
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

func TestReviewAKDoubleDashAsAFlagValueIsNotTheTerminator(t *testing.T) {
	cases := []struct {
		args     []string
		wantEnv  string
		wantVerb bool
		wantArgs []string
	}{
		{[]string{"leaf", "--env", "--", "foo", "--verbose"}, "--", true, []string{"foo"}},
		{[]string{"leaf", "a", "--", "-x", "-y"}, "", false, []string{"a", "-x", "-y"}},
		{[]string{"leaf", "--env=--", "a", "--", "--verbose"}, "--", false, []string{"a", "--verbose"}},
		{[]string{"leaf", "--verbose", "--", "a", "-x"}, "", true, []string{"a", "-x"}},
		{[]string{"leaf", "-", "x", "--", "-verbose"}, "", false, []string{"-", "x", "-verbose"}},
		{[]string{"leaf", "foo", "--verbose", "--", "x"}, "", true, []string{"foo", "x"}},
	}
	for _, tc := range cases {
		fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
		env := fs.String("env", "", "Env")
		verbose := fs.Bool("verbose", false, "Verbose")
		var got []string
		root := &Command{Name: "app", Config: &Config{Out: io.Discard},
			SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func(args []string) error { got = args; return nil }}}}
		if err := root.Execute(tc.args); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if *env != tc.wantEnv || *verbose != tc.wantVerb || !slices.Equal(got, tc.wantArgs) {
			t.Errorf("%v: env=%q verbose=%v args=%v", tc.args, *env, *verbose, got)
		}
	}
}

func TestReviewATHelpAfterAPositionalStillShowsHelp(t *testing.T) {
	for _, args := range [][]string{{"leaf", "a", "--help", "--", "x"}, {"leaf", "--verbose", "--help", "foo", "--bad"}} {
		fs := flag.NewFlagSet("leaf", flag.ContinueOnError)
		fs.Bool("verbose", false, "Verbose")
		ran := false
		var out bytes.Buffer
		root := &Command{Name: "app", Config: &Config{Out: &out},
			SubCommands: []*Command{{Name: "leaf", Flags: fs, Run: func([]string) error { ran = true; return nil }}}}
		if err := root.Execute(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if ran || !strings.Contains(out.String(), "LEARN MORE") {
			t.Errorf("%v: ran=%v, want help", args, ran)
		}
	}
}
