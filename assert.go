package apocheck

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/buger/goterm"
	"github.com/smartystreets/goconvey/convey"
	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
)

type assertionError struct {
	msg         string
	description string
	Expected    interface{}
	Actual      interface{}
}

func newassertionError(msg string) assertionError {
	return assertionError{
		msg: msg,
	}
}

func (e assertionError) Error() string {
	if e.Expected != nil && e.Actual != nil {
		return goterm.Color(fmt.Sprintf("[FAIL] %s: expected: '%s', actual '%s'", e.msg, e.Expected, e.Actual), goterm.RED)
	}
	return goterm.Color(fmt.Sprintf("[FAIL] %s: %s", e.msg, e.description), goterm.RED)
}

// Assert can use goconvey function to perform an assertion.
func Assert(t TestInfo, message string, actual interface{}, f func(interface{}, ...interface{}) string, expected ...interface{}) {

	if msg := f(actual, expected...); msg != "" {

		r := newassertionError(message)

		if err := json.Unmarshal([]byte(msg), &r); err != nil {
			r.description = strings.Replace(strings.Replace(msg, "\n", ", ", -1), "\t", " ", -1)
		}

		panic(r)
	}

	fmt.Fprintf(t, goterm.Color(fmt.Sprintf("- [PASS] %s", message), goterm.GREEN)) // nolint
	fmt.Fprintln(t)                                                                 // nolint
}

// AssertPush asserts a push is correctly received.
func AssertPush(
	ctx context.Context,
	t TestInfo,
	m manipulate.Manipulator,
	identity elemental.Identity,
	eventType elemental.EventType,
	assertEventFunc func(event *elemental.Event, identifiable elemental.Identifiable),
	options ...maniphttp.SubscriberOption,
) func() {

	subctx, cancel := context.WithTimeout(ctx, 10*time.Second)

	evtch := make(chan *elemental.Event)
	errCh := listenForPushEvent(subctx, m, func(evt *elemental.Event) bool {
		if evt.Identity != identity.Name || evt.Type != eventType {
			return false
		}
		go func() { evtch <- evt }()
		return true

	}, options...)

	return func() {

		defer cancel()

		err := <-errCh
		Assert(t, "error is nil", err, convey.ShouldBeNil)

		var evt *elemental.Event

		select {
		case evt = <-evtch:
		case <-time.After(3 * time.Second):
			Assert(t, fmt.Sprintf("should receive a %s event for '%s'", eventType, identity.Name), evt, convey.ShouldNotBeNil)
		case <-subctx.Done():
			return
		}

		Assert(t, fmt.Sprintf("should receive a %s event for '%s'", eventType, identity.Name), evt, convey.ShouldNotBeNil)

		obj := gaia.Manager().Identifiable(identity)
		err = evt.Decode(obj)

		Assert(t, "event is decodable", err, convey.ShouldBeNil)

		if assertEventFunc != nil {
			assertEventFunc(evt, obj)
		}
	}
}

// AssertNoPush asserts a push is not received.
func AssertNoPush(
	ctx context.Context,
	t TestInfo,
	m manipulate.Manipulator,
	identity elemental.Identity,
	eventType elemental.EventType,
	options ...maniphttp.SubscriberOption,
) func() {

	subctx, cancel := context.WithTimeout(ctx, 3*time.Second)

	errCh := listenForPushEvent(subctx, m, func(evt *elemental.Event) bool {
		return evt.Identity == identity.Name && evt.Type == eventType
	}, options...)

	return func() {
		defer cancel()

		err := <-errCh
		Assert(t, fmt.Sprintf("should not receive a %s event for '%s'", eventType, identity.Name), err, convey.ShouldNotBeNil)
	}
}

// Step runs a particular step.
func Step(t TestInfo, name string, step func() error) {

	start := time.Now()
	fmt.Fprintf(t, "%s\n", name) // nolint
	if err := step(); err != nil {
		Assert(t, "step should not return any error", err, convey.ShouldBeNil)
	}

	fmt.Fprintf(t, "%s\n\n", goterm.Color(fmt.Sprintf("took: %s", time.Since(start).Round(time.Millisecond)), goterm.BLUE)) // nolint
}
