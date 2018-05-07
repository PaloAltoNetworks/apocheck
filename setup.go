package apocheck

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aporeto-inc/gaia/v1/golang"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/manipulate/maniphttp"
	"github.com/aporeto-inc/midgard-lib/client"
)

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
func CreateNamespaces(ctx context.Context, m manipulate.Manipulator, rootNamespace string, nss string) (cleanup func() error, err error) {

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
func CreateTestAccount(ctx context.Context, t TestInfo) (manipulate.Manipulator, func() error, error) {

	return CreateAccount(ctx, t.RootManipulator(), t.Account("Euphrates123#", "integ@aporeto.com"))
}

// CreateAccount creates the given account and returns an authenticated manipulator.
func CreateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account) (manipulate.Manipulator, func() error, error) {

	// Keep a reference as create will erase it.
	password := account.Password

	if err := m.Create(nil, account); err != nil {
		return nil, nil, err
	}

	api := maniphttp.ExtractEndpoint(m)
	tlsConfig := maniphttp.ExtractTLSConfig(m)

	c := midgardclient.NewClientWithTLS(api, tlsConfig)

	subctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := c.IssueFromVince(subctx, account.Name, password, "", 5*time.Minute)
	if err != nil {
		return nil, nil, err
	}

	return maniphttp.NewHTTPManipulatorWithTLS(api, "Bearer", token, "/"+account.Name, tlsConfig),
		func() error { return m.Delete(nil, account) },
		nil
}
