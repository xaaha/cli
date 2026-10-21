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
