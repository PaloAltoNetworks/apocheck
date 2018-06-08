package apocheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia/v1/golang"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
	"go.aporeto.io/midgard-lib/client"
)

// Cleanup function is a type function
type Cleanup func() error

// CreateTestNamespace a namespace using the given TestInfo.
func CreateTestNamespace(ctx context.Context, t TestInfo) (string, Cleanup, error) {

	testns := fmt.Sprintf("/%s/%s-%d", t.AccountName(), t.testID, t.iteration)

	clear, err := CreateNamespaces(ctx, t.RootManipulator(), "/"+t.AccountName(), fmt.Sprintf("%s-%d", t.testID, t.iteration))
	if err != nil {
		return "", nil, err
	}

	return testns, clear, nil
}

// CreateNamespaces creates the desired namespace line.
func CreateNamespaces(ctx context.Context, m manipulate.Manipulator, rootNamespace string, nss string) (c Cleanup, err error) {

	var firstns *gaia.Namespace
	mctx := manipulate.NewContext()
	chain := strings.Split(nss, "/")

	for _, name := range chain {

		if name == "" {
			continue
		}

		ns := &gaia.Namespace{Name: name}
		if firstns == nil {
			firstns = ns
		}

		mctx.Namespace = rootNamespace
		if err = m.Create(mctx, ns); err != nil {
			return nil, err
		}
		rootNamespace = ns.Name
	}

	return func() error { return m.Delete(nil, firstns) }, nil
}

// CreateTestAccount creates an account using the given TestInfo and returns an authenticated manipulator.
func CreateTestAccount(ctx context.Context, t TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	return CreateAccount(ctx, t.RootManipulator(), t.Account("Euphrates123#"))
}

// CreateAccount creates the given account and returns an authenticated manipulator.
func CreateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	// Keep a reference as create will erase it.
	password := account.Password

	if err := m.Create(nil, account); err != nil {
		return nil, nil, nil, err
	}

	api := maniphttp.ExtractEndpoint(m)
	tlsConfig := maniphttp.ExtractTLSConfig(m)

	c := midgardclient.NewClientWithTLS(api, tlsConfig)

	subctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := c.IssueFromVince(subctx, account.Name, password, "", 5*time.Minute)
	if err != nil {
		return nil, nil, nil, err
	}

	return maniphttp.NewHTTPManipulatorWithTLS(api, "Bearer", token, "/"+account.Name, tlsConfig),
		account,
		func() error { return m.Delete(nil, account) },
		nil
}

// CheckEnforcersAreUp checks if the enforcers in the given namespace are up
func CheckEnforcersAreUp(ctx context.Context, m manipulate.Manipulator, namespace string) bool {

	mctx := manipulate.NewContext()
	mctx.Namespace = namespace

	enforcers := gaia.EnforcersList{}

	retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := manipulate.Retry(retryCtx, func() error { return m.RetrieveMany(mctx, &enforcers) }, nil)
	if err != nil {
		return false
	}

	for _, enforcer := range enforcers {
		fmt.Println("Enforcer" + enforcer.OperationalStatus)
		if enforcer.OperationalStatus != gaia.EnforcerOperationalStatusConnected {
			return false
		}
	}

	return true
}

// PublicManipulator returns a manipulator facing plublic API from the given manipulator.
func PublicManipulator(t TestInfo, m manipulate.Manipulator, namespace string) manipulate.Manipulator {

	tlsConfig := maniphttp.ExtractTLSConfig(m)
	tlsConfig.Certificates = nil
	tlsConfig.RootCAs = t.PlatformInfo().RootCAPool

	return PublicManipulatorWithTLSConfig(t, m, namespace, tlsConfig)
}

// PublicManipulatorWithTLSConfig returns a manipulator facing plublic API from the given manipulator.
func PublicManipulatorWithTLSConfig(t TestInfo, m manipulate.Manipulator, namespace string, tlsConfig *tls.Config) manipulate.Manipulator {

	username, token := maniphttp.ExtractCredentials(m)

	return maniphttp.NewHTTPManipulatorWithTLS(t.PublicAPI(), username, token, namespace, tlsConfig)
}

// WaitForPushEvent is waiting for a specific event notification.
// Important: highwind and vince needs to set apiPath parameter to `/highwind/events` and `/vince/events`
// For other events, set apiPath to ""
func WaitForPushEvent(ctx context.Context, m manipulate.Manipulator, apiPath string, recursive bool, isWaitingFor func(*elemental.Event) bool) error {

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
