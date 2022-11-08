package apocheck

import (
	"context"
	"fmt"
	"time"

	"github.com/PaloAltoNetworks/barrier"
	"github.com/smartystreets/goconvey/convey"
	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
)

type assertPushConfig struct {
	subscriberOptions    []maniphttp.SubscriberOption
	additionalFilterFunc func(evt *elemental.Event) bool
	assertEventFunc      func(event *elemental.Event, identifiable elemental.Identifiable) error
	positiveTimeout      time.Duration
	negativeTimeout      time.Duration
}

func newAssertPushConfig() assertPushConfig {
	return assertPushConfig{
		positiveTimeout: 120 * time.Second,
		negativeTimeout: 3 * time.Second,
	}
}

// An AssertPushOption represents options to push assertion functions.
type AssertPushOption func(*assertPushConfig)

// AssertPushOptionEventAsserter sets the function to run to validate
// the event and its content after the filter matches.
func AssertPushOptionEventAsserter(asserter func(event *elemental.Event, identifiable elemental.Identifiable) error) func(cfg *assertPushConfig) {
	return func(cfg *assertPushConfig) {
		cfg.assertEventFunc = asserter
	}
}

// AssertPushOptionAdditionalFilter sets an additional filter
// to basic elemental.Identity and elemental.EventType matching.
// If it returns false, the push assertion function continues to wait.
func AssertPushOptionAdditionalFilter(filter func(event *elemental.Event) bool) func(cfg *assertPushConfig) {
	return func(cfg *assertPushConfig) {
		cfg.additionalFilterFunc = filter
	}
}

// AssertPushOptionSubscriberOptions passes additional maniphttp.SuscriberOptions
// to the underlying maniphttp.Subscriber.
func AssertPushOptionSubscriberOptions(options ...maniphttp.SubscriberOption) func(cfg *assertPushConfig) {
	return func(cfg *assertPushConfig) {
		cfg.subscriberOptions = options
	}
}

// AssertPushOptionPositiveTimeout sets the time to wait for a push assertion
// that should find a push.Default is 10s.
func AssertPushOptionPositiveTimeout(timeout time.Duration) func(cfg *assertPushConfig) {
	return func(cfg *assertPushConfig) {
		cfg.positiveTimeout = timeout
	}
}

// AssertPushOptionNegativeTimeout sets the time to wait for a push assertion
// that should not find a push. Default is 3s.
func AssertPushOptionNegativeTimeout(timeout time.Duration) func(cfg *assertPushConfig) {
	return func(cfg *assertPushConfig) {
		cfg.negativeTimeout = timeout
	}
}

// AssertPush asserts a push is correctly received.
func AssertPush(
	ctx context.Context,
	t barrier.TestInfo,
	m manipulate.Manipulator,
	identity elemental.Identity,
	eventType elemental.EventType,
	options ...AssertPushOption,
) func() func() error {

	cfg := newAssertPushConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	subctx, cancel := context.WithTimeout(ctx, cfg.positiveTimeout)

	evtch := make(chan *elemental.Event)
	err := ListenForPushEvent(
		subctx,
		m,
		func(evt *elemental.Event) bool {
			if evt.Identity != identity.Name || evt.Type != eventType {
				return false
			}
			if cfg.additionalFilterFunc != nil {
				if !cfg.additionalFilterFunc(evt) {
					return false
				}
			}
			return true
		},
		evtch,
		cfg.subscriberOptions...,
	)

	barrier.Assert(t, fmt.Sprintf("connecting to events channel for '%s' event for '%s' should work", eventType, identity.Name), err, convey.ShouldBeNil)

	return func() func() error {

		return func() error {
			defer cancel()

			var evt *elemental.Event

			select {
			case evt = <-evtch:
			case <-subctx.Done():
				return fmt.Errorf("did not receive a '%s' event for '%s' in time", eventType, identity.Name)
			}

			obj := gaia.Manager().Identifiable(identity)
			if err := evt.Decode(obj); err != nil {
				return err
			}

			if cfg.assertEventFunc == nil {
				return nil
			}

			return cfg.assertEventFunc(evt, obj)
		}
	}
}

// AssertNoPush asserts a push is not received.
func AssertNoPush(
	ctx context.Context,
	t barrier.TestInfo,
	m manipulate.Manipulator,
	identity elemental.Identity,
	eventType elemental.EventType,
	options ...AssertPushOption,
) func() func() error {

	cfg := newAssertPushConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	subctx, cancel := context.WithTimeout(ctx, cfg.negativeTimeout)

	evtch := make(chan *elemental.Event)
	err := ListenForPushEvent(
		subctx,
		m,
		func(evt *elemental.Event) bool {
			if evt.Identity != identity.Name || evt.Type != eventType {
				return false
			}
			if cfg.additionalFilterFunc != nil {
				if !cfg.additionalFilterFunc(evt) {
					return false
				}
			}
			return true
		},
		evtch,
		cfg.subscriberOptions...,
	)

	barrier.Assert(t, "connecting to events channel should work", err, convey.ShouldBeNil)

	return func() func() error {
		return func() error {
			defer cancel()

			select {
			case <-evtch:
				return fmt.Errorf("received an '%s' event for '%s'", eventType, identity.Name)
			case <-subctx.Done():
				return nil
			}
		}
	}
}

// ListenForPushEvent listen for a event
func ListenForPushEvent(ctx context.Context, m manipulate.Manipulator, verifier func(*elemental.Event) bool, evtCh chan *elemental.Event, options ...maniphttp.SubscriberOption) error {

	subscriber := maniphttp.NewSubscriber(m, options...)
	subscriber.Start(ctx, nil)

	if err := func() error {
		for {
			select {
			case st := <-subscriber.Status():
				if st == manipulate.SubscriberStatusInitialConnection {
					return nil
				}

			case err := <-subscriber.Errors():
				return err

			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}(); err != nil {
		return fmt.Errorf("unable to connect to event channel: %s", err)
	}

	go func() {
		for {
			select {
			case evt := <-subscriber.Events():
				if verifier(evt) {
					if evtCh != nil {
						evtCh <- evt
					}
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
