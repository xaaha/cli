package cli

import (
	"flag"
	"strings"
	"testing"
)

func kapiTree() *Command {
	watchFlags := flag.NewFlagSet("watch", flag.ContinueOnError)
	watchFlags.String("reader", "agent", "Reader name")
	pinFlags := flag.NewFlagSet("pin", flag.ContinueOnError)
	pinFlags.Bool("top", false, "Pin to the top")
	noop := func([]string) error { return nil }
	return &Command{
		Name: "kapi",
		SubCommands: []*Command{
			{Name: "watch", Aliases: []string{"w"}, Short: "Stream events", Flags: watchFlags, Run: noop},
			{Name: "page", Short: "Page actions", SubCommands: []*Command{
				{Name: "pin", Short: "Pin a page", Flags: pinFlags, Run: noop},
			}},
		},
	}
}

func TestD4FlagErrorNamesFullCommandPath(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"watch", "--bad"}, "See 'kapi watch --help' for usage"},
		{[]string{"w", "--bad"}, "See 'kapi watch --help' for usage"},
		{[]string{"page", "pin", "--bad"}, "See 'kapi page pin --help' for usage"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			err := kapiTree().Execute(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestD4HelpNamesRootProgram(t *testing.T) {
	out := captureStdout(t, func() {
		if err := kapiTree().Execute([]string{"page", "--help"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Use `kapi <command> --help`") {
		t.Errorf("help does not name the root program:\n%s", out)
	}
	if strings.Contains(out, "hulak") {
		t.Errorf("help mentions hulak:\n%s", out)
	}
}
