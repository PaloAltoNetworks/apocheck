package apocheck

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
	midgardclient "go.aporeto.io/midgard-lib/client"
)

// Cleanup function is a type function.
type Cleanup func() error

// CreateTestAccount creates an account using the given TestInfo and returns an authenticated manipulator.
func CreateTestAccount(ctx context.Context, m manipulate.Manipulator, t TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	account := t.Account("Euphrates123#")
	account.AccessEnabled = true

	return CreateAccount(ctx, m, account, t)
}

// CreateTestNamespace a namespace using the given TestInfo.
func CreateTestNamespace(ctx context.Context, m manipulate.Manipulator, t TestInfo) (string, Cleanup, error) {

	testns := fmt.Sprintf("/%s/%s-%d", t.AccountName(), t.testID, t.iteration)

	clear, err := CreateNamespaces(ctx, m, "/"+t.AccountName(), fmt.Sprintf("%s-%d", t.testID, t.iteration))
	if err != nil {
		return "", nil, err
	}

	return testns, clear, nil
}

// CreateAccount creates the given gaia.Account and returns a manipulator for this account.
func CreateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account, t TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	// Keep a ref as Create qwill reset it.
	password := account.Password

	if err := m.Create(nil, account); err != nil {
		return nil, nil, nil, err
	}

	token, err := midgardclient.NewClientWithTLS(t.publicAPI, t.publicTLSConfig).IssueFromVince(ctx, account.Name, password, "", t.Timeout())
	if err != nil {
		return nil, nil, nil, err
	}

	accountManipulator, _ := maniphttp.New(
		ctx,
		t.publicAPI,
		maniphttp.OptionToken(token),
		maniphttp.OptionNamespace("/"+account.Name),
		maniphttp.OptionTLSConfig(t.publicTLSConfig),
	)

	cleanUpfunc := func() error { return m.Delete(nil, account) }

	return accountManipulator, account, cleanUpfunc, nil
}

// CreateNamespaces creates the desired namespace line.
func CreateNamespaces(ctx context.Context, m manipulate.Manipulator, rootNamespace string, nss string) (c Cleanup, err error) {

	var firstns *gaia.Namespace
	chain := strings.Split(nss, "/")
	var mctx, firstNSmctx manipulate.Context
	for _, name := range chain {

		if name == "" {
			continue
		}

		mctx = manipulate.NewContext(
			ctx,
			manipulate.ContextOptionNamespace(rootNamespace),
		)

		ns := &gaia.Namespace{Name: name, ServiceCertificateValidity: "1h"}
		if firstns == nil {
			firstns = ns
			firstNSmctx = mctx
		}
		if err = m.Create(mctx, ns); err != nil {
			return nil, err
		}
		rootNamespace = ns.Name
	}

	return func() error { return m.Delete(firstNSmctx, firstns) }, nil
}

// WaitForPushEvent is waiting for a specific event notification.
// Important: Vince needs to set apiPath parameter to `/vince/events`
// For other events, set apiPath to ""
func WaitForPushEvent(ctx context.Context, m manipulate.Manipulator, apiPath string, recursive bool, isWaitingFor func(*elemental.Event) bool) error {

	fmt.Println("DEPRECATED: apocheck.WaitForPushEvent is deprecated. Switch to apocheck.ListenForPushEvent")

	var subscriber manipulate.Subscriber

	if apiPath == "" {
		subscriber = maniphttp.NewSubscriber(m, recursive)
	} else {
		subscriber = maniphttp.NewSubscriberWithEndpoint(m, apiPath, recursive)
	}

	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	subscriber.Start(innerCtx, nil)

	for {
		select {
		case err := <-subscriber.Errors():
			return err

		case evt := <-subscriber.Events():
			if isWaitingFor(evt) {
				return nil
			}

		case st := <-subscriber.Status():
			switch st {
			case manipulate.SubscriberStatusInitialConnectionFailure:
				return fmt.Errorf("waiting for push canceled: subscriber status connect failed")
			case manipulate.SubscriberStatusDisconnection:
				return fmt.Errorf("waiting for push canceled: subscriber status disconnected")
			case manipulate.SubscriberStatusFinalDisconnection:
				return fmt.Errorf("waiting for push canceled: subscriber status terminated")
			}

		case <-ctx.Done():
			return fmt.Errorf("waiting for push canceled: %s", ctx.Err())
		}
	}
}

// ListenForPushEvent returns a channel what will contain an error if the push event is not passing the verifier
// in the given timeout, or nil if it does.
func ListenForPushEvent(ctx context.Context, m manipulate.Manipulator, recursive bool, verifier func(*elemental.Event) bool) chan error {

	ch := make(chan error)
	connected := make(chan struct{}, 2)

	send := func(err error) {
		select {
		case ch <- err:
		case <-time.After(5 * time.Second):
			fmt.Printf("apocheck.ListenForPushEvent: unable to publish err: %s\n", err)
		}
	}

	go func() {
		subscriber := maniphttp.NewSubscriber(m, recursive)
		subscriber.Start(ctx, nil)

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
