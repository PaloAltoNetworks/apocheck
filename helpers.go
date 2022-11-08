package apocheck

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/PaloAltoNetworks/barrier"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
	midgardclient "go.aporeto.io/midgard-lib/client"
)

// Cleanup function is a type function.
type Cleanup func() error

// CreateTestAccount creates an account using the given TestInfo and returns an authenticated manipulator.
func CreateTestAccount(ctx context.Context, m manipulate.Manipulator, t barrier.TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	a, ok := t.Stash().(aporeto)
	if !ok {
		log.Fatalln(errors.New("invalid setup"))
	}

	account := a.Account("Euphrates123#")
	account.AccessEnabled = true

	return CreateAccount(ctx, m, account, t)
}

// CreateTestNamespace a namespace using the given TestInfo.
func CreateTestNamespace(ctx context.Context, m manipulate.Manipulator, t barrier.TestInfo) (string, Cleanup, error) {

	a, ok := t.Stash().(aporeto)
	if !ok {
		log.Fatalln(errors.New("invalid setup"))
	}

	testns := fmt.Sprintf("/%s/%s-%d", a.AccountName(), t.TestID(), t.Iteration())

	clear, err := CreateNamespaces(ctx, m, "/"+a.AccountName(), fmt.Sprintf("%s-%d", t.TestID(), t.Iteration()))
	if err != nil {
		return "", nil, err
	}

	return testns, clear, nil
}

// CreateAccount creates the given gaia.Account and returns a manipulator for this account.
func CreateAccount(ctx context.Context, m manipulate.Manipulator, account *gaia.Account, t barrier.TestInfo) (manipulate.Manipulator, *gaia.Account, Cleanup, error) {

	a, ok := t.Stash().(aporeto)
	if !ok {
		log.Fatalln(errors.New("invalid setup"))
	}

	// Keep a ref as Create qwill reset it.
	password := account.Password

	if err := m.Create(nil, account); err != nil {
		return nil, nil, nil, err
	}

	token, err := midgardclient.NewClientWithTLS(a.PublicAPI(), a.PublicTLSConfig()).IssueFromVince(ctx, account.Name, password, "", t.Timeout())
	if err != nil {
		return nil, nil, nil, err
	}

	accountManipulator, _ := maniphttp.New(
		ctx,
		a.PublicAPI(),
		maniphttp.OptionToken(token),
		maniphttp.OptionEncoding(a.encoding),
		maniphttp.OptionNamespace("/"+account.Name),
		maniphttp.OptionTLSConfig(a.PublicTLSConfig()),
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

		ns := gaia.NewNamespace()
		ns.Name = name
		ns.ServiceCertificateValidity = "1h"

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
