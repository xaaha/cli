package cli

import (
	"bytes"
	"flag"
	"slices"
	"strings"
	"testing"
)

func TestReviewABOnlyExactHelpArgumentsAreHelp(t *testing.T) {
	cases := []struct {
		arg      string
		short    string
		wantHelp bool
	}{
		{"--help", "h", true},
		{"-h", "h", true},
		{"help", "h", true},
		{"-", "", false},
		{"--", "", false},
		{"-", "h", false},
		{"---help", "h", false},
		{"--h", "h", false},
		{"-help", "h", false},
		{"--help=false", "h", false},
		{"--helpx", "h", false},
		{"HELP", "h", false},
	}
	for _, tc := range cases {
		t.Run(tc.arg+"/short="+tc.short, func(t *testing.T) {
			var got []string
			ran := false
			var out bytes.Buffer
			root := &Command{
				Name:   "app",
				Config: &Config{Out: &out, FlagAliases: map[string]string{"help": tc.short}},
				SubCommands: []*Command{
					{Name: "cat", Run: func(args []string) error { ran, got = true, args; return nil }},
				},
			}
			if err := root.Execute([]string{"cat", tc.arg}); err != nil {
				t.Fatal(err)
			}
			if tc.wantHelp && (ran || !strings.Contains(out.String(), "LEARN MORE")) {
				t.Errorf("want help, ran=%v", ran)
			}
			if !tc.wantHelp && (!ran || !slices.Equal(got, []string{tc.arg})) {
				t.Errorf("want Run(%q), ran=%v args=%v help=%q", tc.arg, ran, got, out.String())
			}
		})
	}
}

func TestReviewABAppsOwnHelpFlagAsFirstArgument(t *testing.T) {
	for _, arg := range []string{"--help", "-help"} {
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		manual := fs.Bool("help", false, "Show the manual")
		ran := false
		root := &Command{Name: "app", Config: &Config{Out: &bytes.Buffer{}}, SubCommands: []*Command{
			{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
		}}
		if err := root.Execute([]string{"run", arg}); err != nil {
			t.Fatal(err)
		}
		if !ran || !*manual {
			t.Errorf("%s: ran=%v manual=%v, want the app's own flag set", arg, ran, *manual)
		}
	}
}

func TestReviewABHelpWordAtEveryLevel(t *testing.T) {
	leafFlags := flag.NewFlagSet("leaf", flag.ContinueOnError)
	leafFlags.String("out", "", "Out")
	grpFlags := flag.NewFlagSet("grp", flag.ContinueOnError)
	grpFlags.String("env", "", "Env")
	for _, args := range [][]string{{"help"}, {"grp", "help"}, {"grp", "leaf", "help"}, {"grp", "--env", "p", "--help"}} {
		ran := false
		var out bytes.Buffer
		root := &Command{Name: "app", Config: &Config{Out: &out}, SubCommands: []*Command{
			{Name: "grp", Flags: grpFlags, SubCommands: []*Command{
				{Name: "leaf", Flags: leafFlags, Run: func([]string) error { ran = true; return nil }},
			}},
		}}
		if err := root.Execute(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if ran || !strings.Contains(out.String(), "LEARN MORE") {
			t.Errorf("%v: ran=%v, want help", args, ran)
		}
	}
}

func TestReviewAIHelpFalseRunsTheCommand(t *testing.T) {
	ran := false
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	root := &Command{Name: "app", Config: &Config{Out: &bytes.Buffer{}}, SubCommands: []*Command{
		{Name: "run", Flags: fs, Run: func([]string) error { ran = true; return nil }},
	}}
	if err := root.Execute([]string{"run", "--help=false"}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("--help=false should run the command")
	}
}
