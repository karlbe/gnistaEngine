package solve

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/input"
)

// A fixture is a recorded game: the input of every tick, run-length encoded as "value:count"
// pairs separated by spaces. It holds no game data, only what the player pressed.

// EncodeInputs writes the inputs as a fixture.
func EncodeInputs(in []input.Actions) string {
	var sb strings.Builder
	for i := 0; i < len(in); {
		j := i
		for j < len(in) && in[j] == in[i] {
			j++
		}
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%d:%d", in[i], j-i)
		i = j
	}
	return sb.String()
}

// DecodeInputs reads a fixture written by EncodeInputs.
func DecodeInputs(s string) ([]input.Actions, error) {
	var out []input.Actions
	for _, f := range strings.Fields(s) {
		v, n, ok := strings.Cut(f, ":")
		if !ok {
			return nil, fmt.Errorf("bad run %q", f)
		}
		val, err1 := strconv.Atoi(v)
		cnt, err2 := strconv.Atoi(n)
		if err1 != nil || err2 != nil || cnt < 1 {
			return nil, fmt.Errorf("bad run %q", f)
		}
		for i := 0; i < cnt; i++ {
			out = append(out, input.Actions(val))
		}
	}
	return out, nil
}
