package cli

import (
	"bytes"
	"flag"
	"io"
	"slices"
	"strings"
	"testing"
)

func flagsFirstTree(got *[]string, on *string, out io.Writer) *Command {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	fs.StringVar(on, "on", "", "Element id")
	return &Command{Name: "kapi", Config: &Config{Out: out}, SubCommands: []*Command{{
		Name: "say", Flags: fs, FlagsFirst: true,
		Run: func(args []string) error { *got = args; return nil },
	}}}
}

func TestD14FlagsFirstStopsAtTheFirstPositional(t *testing.T) {
	cases := []struct {
		args   []string
		wantOn string
		want   []string
	}{
		{[]string{"say", "--on", "x", "page.html", "-", "item", "-5", "--on", "y"}, "x", []string{"page.html", "-", "item", "-5", "--on", "y"}},
		{[]string{"say", "page.html", "--help"}, "", []string{"page.html", "--help"}},
		{[]string{"say", "--", "-page.html"}, "", []string{"-page.html"}},
	}
	for _, tc := range cases {
		var got []string
		var on string
		if err := flagsFirstTree(&got, &on, io.Discard).Execute(tc.args); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if on != tc.wantOn || !slices.Equal(got, tc.want) {
			t.Errorf("%v: on=%q args=%v, want on=%q args=%v", tc.args, on, got, tc.wantOn, tc.want)
		}
	}
}

func TestD14FlagsFirstStillShowsHelpAndRejectsBadFlags(t *testing.T) {
	var got []string
	var on string
	var out bytes.Buffer
	if err := flagsFirstTree(&got, &on, &out).Execute([]string{"say", "--on", "x", "--help"}); err != nil {
		t.Fatal(err)
	}
	if got != nil || !strings.Contains(out.String(), "LEARN MORE") {
		t.Errorf("want help before any positional, ran with %v", got)
	}
	err := flagsFirstTree(&got, &on, io.Discard).Execute([]string{"say", "--bogus", "page.html"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -bogus") {
		t.Errorf("error = %v", err)
	}
}

func TestD14InterspersedFlagsRemainTheDefault(t *testing.T) {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	on := fs.String("on", "", "Element id")
	var got []string
	root := &Command{Name: "kapi", SubCommands: []*Command{{Name: "say", Flags: fs, Run: func(a []string) error { got = a; return nil }}}}
	if err := root.Execute([]string{"say", "page.html", "--on", "x"}); err != nil {
		t.Fatal(err)
	}
	if *on != "x" || !slices.Equal(got, []string{"page.html"}) {
		t.Errorf("on=%q args=%v", *on, got)
	}
}
