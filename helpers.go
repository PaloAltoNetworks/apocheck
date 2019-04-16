package apocheck

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		maniphttp.OptionEncoding(t.encoding),
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

// CreateNamespace creates the namespace with the given name in the given namespace.
// It returns the created namespace, a manipulate.Context pointing to the namespace and an eventual error.
func CreateNamespace(ctx context.Context, m manipulate.Manipulator, name string, mctx manipulate.Context) (*gaia.Namespace, manipulate.Context, error) {

	ns := gaia.NewNamespace()
	ns.Name = name

	options := []manipulate.ContextOption{}

	if mctx == nil {
		mctx = manipulate.NewContext(ctx, options...)
	}

	if err := m.Create(mctx, ns); err != nil {
		return nil, nil, err
	}

	defer func() { <-time.After(500 * time.Millisecond) }() // let a bit of time for auth caches.

	return ns, manipulate.NewContext(ctx, manipulate.ContextOptionNamespace(ns.Name)), nil
}
