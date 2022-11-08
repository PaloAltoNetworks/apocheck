package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
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

	token, err := midgardclient.NewClientWithTLS(t.PublicAPI(), t.PublicTLSConfig()).IssueFromVince(ctx, account.Name, password, "", t.Timeout())
	if err != nil {
		return nil, nil, nil, err
	}

	accountManipulator, _ := maniphttp.New(
		ctx,
		t.PublicAPI(),
		maniphttp.OptionToken(token),
		maniphttp.OptionEncoding(t.encoding),
		maniphttp.OptionNamespace("/"+account.Name),
		maniphttp.OptionTLSConfig(t.PublicTLSConfig()),
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

// aporeto Specific Code:

type aporeto struct {
	encoding          elemental.EncodingType
	privateAPI        string
	privateTLSConfig  *tls.Config
	rootManipulator   manipulate.Manipulator
	publicAPI         string
	publicTLSConfig   *tls.Config
	publicManipulator manipulate.Manipulator
	testID            string
}

func newAporeto(ctx context.Context) (a aporeto, err error) {

	var caPoolPublic, caPoolPrivate *x509.CertPool
	var systemCert *tls.Certificate

	if path := viper.GetString("cacert-public"); path != "" {
		caPoolPublic, err = setupPublicCA(path)
		if err != nil {
			return a, fmt.Errorf("unable to load public ca from path '%s': %s", path, err)
		}
	}

	if path := viper.GetString("cacert-private"); path != "" {
		caPoolPrivate, err = setupPrivateCA(path)
		if err != nil {
			return a, fmt.Errorf("unable to load private ca from path '%s': %s", path, err)
		}
	}

	if certPath, keyPath := viper.GetString("cert"), viper.GetString("key"); certPath != "" && keyPath != "" {
		systemCert, err = setupCerts(certPath, keyPath, viper.GetString("key-pass"))
		if err != nil {
			return a, err
		}
	}

	var encoding elemental.EncodingType
	switch viper.GetString("encoding") {
	case "json":
		encoding = elemental.EncodingTypeJSON
	case "msgpack":
		encoding = elemental.EncodingTypeMSGPACK
	default:
		return a, fmt.Errorf("invalid encoding" + viper.GetString("encoding"))
	}

	// Token and Namespace
	token := viper.GetString("token")
	namespace := viper.GetString("namespace")

	publicTLSConfig := &tls.Config{
		RootCAs:            caPoolPublic,
		InsecureSkipVerify: true, // nolint
	}

	privateTLSConfig := &tls.Config{}
	if systemCert != nil && caPoolPrivate != nil {
		privateTLSConfig = &tls.Config{
			RootCAs:            caPoolPrivate,
			Certificates:       []tls.Certificate{*systemCert},
			InsecureSkipVerify: true, // nolint
		}
	}

	// Public Manipulator
	publicAPI := viper.GetString("api-public")
	var publicManipulator manipulate.Manipulator
	if token != "" && publicAPI != "" {
		publicManipulator, _ = maniphttp.New(
			ctx,
			publicAPI,
			maniphttp.OptionToken(token),
			maniphttp.OptionNamespace(namespace),
			maniphttp.OptionEncoding(encoding),
			maniphttp.OptionTLSConfig(publicTLSConfig),
		)
	}

	// private manipulator
	privateAPI := viper.GetString("api-private")
	var rootManipulator manipulate.Manipulator
	if systemCert != nil && privateAPI != "" {
		rootManipulator, _ = maniphttp.New(
			ctx,
			privateAPI,
			maniphttp.OptionNamespace(namespace),
			maniphttp.OptionTLSConfig(privateTLSConfig),
			maniphttp.OptionEncoding(encoding),
		)
	}

	return aporeto{
		// Aporeto Specific
		encoding:          encoding,
		privateAPI:        privateAPI,
		privateTLSConfig:  privateTLSConfig,
		rootManipulator:   rootManipulator,
		publicAPI:         publicAPI,
		publicTLSConfig:   publicTLSConfig,
		publicManipulator: publicManipulator,
	}, nil
}

// RootManipulator returns the root manipulator if any.
func (a *aporeto) RootManipulator() manipulate.Manipulator {
	return a.rootManipulator
}

// PublicManipulator returns the public manipulator if any.
func (a *aporeto) PublicManipulator() manipulate.Manipulator {
	return a.publicManipulator
}

// PublicAPI returns the public API endpoina.
func (a *aporeto) PublicAPI() string {
	return a.publicAPI
}

// PrivateAPI returns the private API endpoina.
func (a *aporeto) PrivateAPI() string {
	return a.privateAPI
}

// PublicTLSConfig returns the public TLS config.
func (a *aporeto) PublicTLSConfig() *tls.Config {
	return a.publicTLSConfig
}

// PrivateTLSConfig returns the public TLS config.
func (a *aporeto) PrivateTLSConfig() *tls.Config {
	return a.privateTLSConfig
}

// Account returns a gaia Account object that can be used for the test.
func (a *aporeto) Account(password string) *gaia.Account {

	// nolint
	return &gaia.Account{
		Name:     a.AccountName(),
		Password: password,
		Email:    fmt.Sprintf("user@%s.com", a.AccountName()),
	}
}

// TestNamespace returns a unique namespace that can be used by this test.
func (a *aporeto) TestNamespace(iteration int) string {
	return fmt.Sprintf("/%s/%s", a.AccountName(), a.testID)
}

// AccountName returns a unique account name that can be used by this test.
func (a *aporeto) AccountName() string {
	return fmt.Sprintf("account-%s", a.testID)
}

// AccountNamespace returns the account namespace that can be used by this test.
func (a *aporeto) AccountNamespace() string {
	return fmt.Sprintf("/account-%s", a.testID)
}
