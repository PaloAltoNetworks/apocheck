package apocheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/aporeto-inc/gaia/v1/golang"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/manipulate/maniphttp"
	"github.com/aporeto-inc/midgard-lib/client"
)

type Cleanup func() error

// CreateTestNamespace a namespace using the given TestInfo.
func CreateTestNamespace(ctx context.Context, t TestInfo) (string, func() error, error) {

	testns := fmt.Sprintf("/%s/%s-%d", t.AccountName(), t.testID, t.iteration)

	clear, err := CreateNamespaces(ctx, t.RootManipulator(), "/"+t.AccountName(), fmt.Sprintf("%s-%d", t.testID, t.iteration))
	if err != nil {
		return "", nil, err
	}

	return testns, clear, nil
}

// CreateNamespaces creates the desired namespace line.
func CreateNamespaces(ctx context.Context, m manipulate.Manipulator, rootNamespace string, nss string) (c Cleanup, err error) {

	mctx := manipulate.NewContext()
	chain := strings.Split(nss, "/")
	firstns := &gaia.Namespace{}

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
func CreateTestAccount(ctx context.Context, t TestInfo) (manipulate.Manipulator, *gaia.Account, func() error, error) {

	return CreateAccount(ctx, t.RootManipulator(), t.Account("Euphrates123#"))
}

// CreateAccount creates the given account and returns an authenticated manipulator.
func CreateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account) (manipulate.Manipulator, *gaia.Account, func() error, error) {

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
