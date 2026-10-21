package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
	"time"
)

type endpointValue string

func (e *endpointValue) String() string     { return string(*e) }
func (e *endpointValue) Set(s string) error { *e = endpointValue(s); return nil }

func TestReviewBBTypeHintsMatchTheFlagType(t *testing.T) {
	var ep endpointValue
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Var(&ep, "endpoint", "Endpoint")
	fs.Int("count", 0, "Count")
	fs.Int64("big", 0, "Big")
	fs.Uint("workers", 0, "Workers")
	fs.Float64("ratio", 0, "Ratio")
	fs.Duration("wait", time.Second, "Wait")
	fs.String("name", "", "Name")
	var out bytes.Buffer
	(&Command{Name: "run", Flags: fs, Config: &Config{Out: &out}}).PrintHelp()
	for _, want := range []string{"-endpoint string", "-count int", "-big int", "-workers uint", "-ratio float", "-wait duration", "-name string"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q:\n%s", want, out.String())
		}
	}
}
