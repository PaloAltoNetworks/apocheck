package apocheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
	midgardclient "go.aporeto.io/midgard-lib/client"
)

// Cleanup function is a type function.
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

// CreateTestAccount creates an account using the given TestInfo and returns an authenticated manipulator.
func CreateTestAccount(ctx context.Context, t TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {
	account := t.Account("Euphrates123#")
	account.AccessEnabled = true

	return CreateAccount(ctx, t.RootManipulator(), account, t.Timeout())
}

// CreateAccount creates the given account and returns an authenticated manipulator.
func CreateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account, timeout time.Duration) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	// Keep a reference as create will erase it.
	password := account.Password

	a, _, err := CreateUnauthenticatedAccount(ctx, m, account)
	if err != nil {
		return nil, nil, nil, err
	}

	m, err = AuthenticateAccount(ctx, m, a, password, timeout)
	if err != nil {
		return nil, nil, nil, err
	}

	return m,
		a,
		func() error { return m.Delete(nil, a) },
		nil
}

// AuthenticateAccount authenticates an account by issuing a token from midgard.
func AuthenticateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account, password string, tokenValidity time.Duration) (manipulate.Manipulator, error) {

	endpoint := maniphttp.ExtractEndpoint(m)
	tlsConfig := maniphttp.ExtractTLSConfig(m)
	tlsConfig.Certificates = nil

	c := midgardclient.NewClientWithTLS(endpoint, tlsConfig)

	subctx, cancel := context.WithTimeout(ctx, 10*time.Second)

	defer cancel()

	token, err := c.IssueFromVince(subctx, account.Name, password, "", tokenValidity)
	if err != nil {
		return nil, err
	}

	return maniphttp.NewHTTPManipulatorWithTLS(endpoint, "Bearer", token, "/"+account.Name, tlsConfig),
		nil
}

// CreateUnauthenticatedTestAccount creates an account with invited status using the given TestInfo and returns an authenticated manipulator.
func CreateUnauthenticatedTestAccount(ctx context.Context, t TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {
	a, deleteaccount, err := CreateUnauthenticatedAccount(ctx, t.RootManipulator(), t.Account("Euphrates123#"))
	return t.RootManipulator(), a, deleteaccount, err
}

// CreateUnauthenticatedAccount creates the given account and returns an authenticated manipulator.
func CreateUnauthenticatedAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account) (*gaia.Account, Cleanup, error) {

	if err := m.Create(nil, account); err != nil {
		return nil, nil, err
	}

	return account,
		func() error { return m.Delete(nil, account) },
		nil
}

// CheckIfGivenEnforcerIsUp checks if the given enforcer in the given namespace is up.
func CheckIfGivenEnforcerIsUp(ctx context.Context, m manipulate.Manipulator, namespace, enforcerName string) error {

	mctx := manipulate.NewContext(
		ctx,
		manipulate.ContextOptionNamespace(namespace),
		manipulate.ContextOptionFilter(
			manipulate.
				NewFilterComposer().WithKey("name").
				Equals(enforcerName).
				Done(),
		),
	)

	enforcers := gaia.EnforcersList{}

	err := m.RetrieveMany(mctx, &enforcers)
	if err != nil {
		return err
	}

	if len(enforcers) == 0 {
		return fmt.Errorf("no enforcers found")
	}

	if len(enforcers) > 1 {
		panic("found more than one enforcer with same name")
	}

	if enforcers[0].OperationalStatus != gaia.EnforcerOperationalStatusConnected {
		return fmt.Errorf("enforcer status: %s", enforcers[0].OperationalStatus)
	}

	return nil
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

// NewUUID returns a new UUID in string form
func NewUUID() string {
	return uuid.Must(uuid.NewV4(), nil).String()
}
