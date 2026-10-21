package cli

import (
	"errors"
	"io"
	"testing"
)

type failAfter struct{ n int }

var errDiskFull = errors.New("disk full")

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errDiskFull
	}
	f.n--
	return len(p), nil
}

func TestReviewPCompletionWritersReturnWriteErrors(t *testing.T) {
	for shell, write := range map[string]func(io.Writer, *Command) error{"zsh": WriteZshCompletion, "bash": WriteBashCompletion} {
		for _, okWrites := range []int{0, 5} {
			if err := write(&failAfter{n: okWrites}, fixtureTree()); !errors.Is(err, errDiskFull) {
				t.Errorf("%s after %d writes: error = %v, want %v", shell, okWrites, err, errDiskFull)
			}
		}
	}
}
