package cli

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestReviewAFHelpFlagsAddedByAnotherTreeStillCount(t *testing.T) {
	shared := flag.NewFlagSet("shared", flag.ContinueOnError)
	mk := func(alias string, out io.Writer, ran *bool) *Command {
		return &Command{Name: "app", Config: &Config{Out: out, FlagAliases: map[string]string{"help": alias}},
			SubCommands: []*Command{{Name: "leaf", Flags: shared, Run: func([]string) error { *ran = true; return nil }}}}
	}
	var ranA, ranB bool
	if err := mk("h", io.Discard, &ranA).Execute([]string{"leaf"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := mk("H", &out, &ranB).Execute([]string{"leaf", "x", "-h"}); err != nil {
		t.Fatal(err)
	}
	if ranB || !strings.Contains(out.String(), "LEARN MORE") {
		t.Errorf("-h added by the other tree was ignored: ran=%v", ranB)
	}
}
