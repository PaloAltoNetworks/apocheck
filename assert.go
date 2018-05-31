package apocheck

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/buger/goterm"
	"github.com/smartystreets/goconvey/convey"
)

type res struct {
	Actual   interface{}
	Expected interface{}
}

type assestionError struct {
	msg         string
	description string
	Expected    interface{}
	Actual      interface{}
}

func newAssestionError(msg string) assestionError {
	return assestionError{
		msg: msg,
	}
}

func (e assestionError) Error() string {
	if e.Expected != nil && e.Actual != nil {
		return goterm.Color(fmt.Sprintf("[FAIL] %s: expected: '%s', actual '%s'", e.msg, e.Expected, e.Actual), goterm.RED)
	}
	return goterm.Color(fmt.Sprintf("[FAIL] %s: %s", e.msg, e.description), goterm.RED)
}

// Assert can use goconvey function to perform an assertion.
func Assert(t TestInfo, message string, actual interface{}, f func(interface{}, ...interface{}) string, expected ...interface{}) {

	var w io.Writer
	w = t
	if msg := f(actual, expected...); msg != "" {

		r := newAssestionError(message)

		if err := json.Unmarshal([]byte(msg), &r); err != nil {
			r.description = strings.Replace(strings.Replace(msg, "\n", ", ", -1), "\t", " ", -1)
		}

		panic(r)
	}

	fmt.Fprintf(w, goterm.Color(fmt.Sprintf("- [PASS] %s (%s)", message, t.TimeSinceLastStep()), goterm.GREEN)) // nolint
	fmt.Fprintln(w)
}

// Step runs a particular step.
func Step(t TestInfo, name string, step func() error) {

	fmt.Fprintf(t, "%s (%s)\n", name, t.TimeSinceLastStep())
	if err := step(); err != nil {
		Assert(t, "step should not return any error", err, convey.ShouldBeNil)
	}
}
