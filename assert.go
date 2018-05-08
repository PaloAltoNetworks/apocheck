package apocheck

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/buger/goterm"
)

type res struct {
	Actual   interface{}
	Expected interface{}
}

type assestionError struct {
	msg      string
	Expected interface{}
	Actual   interface{}
}

func newAssestionError(msg string) assestionError {
	return assestionError{
		msg: msg,
	}
}

func (e assestionError) Error() string {
	return goterm.Color(
		fmt.Sprintf("[FAIL] %s: expected: '%s', actual '%s'", e.msg, e.Expected, e.Actual), goterm.RED)
}

// Assert can use goconvey function to perform an assertion.
func Assert(w io.Writer, message string, actual interface{}, f func(interface{}, ...interface{}) string, expected ...interface{}) {

	if msg := f(actual, expected...); msg != "" {

		r := newAssestionError(message)
		if err := json.Unmarshal([]byte(msg), &r); err != nil {
			panic(goterm.Color(fmt.Sprintf("[FAIL] unable to decode assertion result: %s", err), goterm.RED))
		}
		panic(r)
	}

	fmt.Fprintf(w, goterm.Color(fmt.Sprintf("- [PASS] %s", message), goterm.GREEN)) // nolint
	fmt.Fprintln(w)
}
