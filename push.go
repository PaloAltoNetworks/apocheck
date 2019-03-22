package apocheck

import (
	"context"
	"fmt"
	"time"

	"go.aporeto.io/elemental"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
)

// ListenForPushEvent listen for a event
func ListenForPushEvent(ctx context.Context, m manipulate.Manipulator, verifier func(*elemental.Event) bool, options ...maniphttp.SubscriberOption) chan error {

	ch := make(chan error)
	connected := make(chan struct{}, 2)

	send := func(err error) {
		select {
		case ch <- err:
		case <-time.After(10 * time.Second):
			fmt.Printf("apocheck.ListenForPushEvent: unable to publish err: %s\n", err)
		}
	}

	subscriber := maniphttp.NewSubscriber(m, options...)
	subscriber.Start(ctx, nil)

	go func() {

		for {
			select {
			case err := <-subscriber.Errors():
				send(err)
				return

			case evt := <-subscriber.Events():
				if verifier(evt) {
					send(nil)
					return
				}

			case st := <-subscriber.Status():
				switch st {

				case manipulate.SubscriberStatusInitialConnection:
					connected <- struct{}{}

				case manipulate.SubscriberStatusInitialConnectionFailure:
					send(fmt.Errorf("waiting for push canceled: subscriber status connect failed"))
					return

				case manipulate.SubscriberStatusDisconnection:
					send(fmt.Errorf("waiting for push canceled: subscriber status disconnected"))
					return
				}

			case <-ctx.Done():
				send(fmt.Errorf("waiting for push canceled: %s", ctx.Err()))
				return
			}
		}
	}()

	select {
	case <-connected:
	case <-ctx.Done():
		go func() { send(fmt.Errorf("unable to connect to push channel: timeout")) }()
	}

	return ch
}
