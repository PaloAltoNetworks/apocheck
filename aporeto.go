package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
	"go.aporeto.io/tg/tglib"
)

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

// New creates a new aporeto object that can be stashed into barrier.
func New(ctx context.Context) (a aporeto, err error) {

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

func setupPublicCA(caPublicPath string) (*x509.CertPool, error) {

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if caPublicPath != "" {
		data, err := os.ReadFile(caPublicPath)
		if err != nil {
			return nil, err
		}

		pool.AppendCertsFromPEM(data)
	}

	return pool, nil
}

func setupPrivateCA(caSystemPath string) (*x509.CertPool, error) {

	data, err := os.ReadFile(caSystemPath)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(data)

	return pool, nil
}

func setupCerts(certPath string, keyPath string, keyPass string) (*tls.Certificate, error) {

	x509Cert, key, err := tglib.ReadCertificatePEM(certPath, keyPath, keyPass)
	if err != nil {
		return nil, err
	}

	cert, err := tglib.ToTLSCertificate(x509Cert, key)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

// ExtendArgs extends the commands with Aporeto specific args
func ExtendArgs(c *cobra.Command) {

	defaultCaCertPrivate := ""
	defaultCert := ""
	defaultKey := ""
	defaultCaCertPublic := ""
	cf := os.Getenv("CERTS_FOLDER")
	if cf != "" {
		defaultCaCertPrivate = os.ExpandEnv("$CERTS_FOLDER/ca-chain-system.pem")
		defaultCert = os.ExpandEnv("$CERTS_FOLDER/system-cert.pem")
		defaultKey = os.ExpandEnv("$CERTS_FOLDER/system-key.pem")
		defaultCaCertPublic = os.ExpandEnv("$CERTS_FOLDER/ca-chain-public.pem")
	}
	// Parameters to connect to private API
	c.PersistentFlags().String("api-private", "https://127.0.0.1:4444", "Address of the private api gateway")
	c.PersistentFlags().String("cacert-private", defaultCaCertPrivate, "Path to the private api ca certificate")
	c.PersistentFlags().String("cert", defaultCert, "Path to client certificate")
	c.PersistentFlags().String("key-pass", "", "Password for the certificate key")
	c.PersistentFlags().String("key", defaultKey, "Path to client certificate key")

	// Parameters to connect to public API
	c.PersistentFlags().String("api-public", "https://127.0.0.1:4443", "Address of the public api gateway")
	c.PersistentFlags().String("cacert-public", defaultCaCertPublic, "Path to the public api ca certificate")
	c.PersistentFlags().String("token", "", "Access Token")
	c.PersistentFlags().String("namespace", "/", "Account Name")

	// Encoding
	c.PersistentFlags().String("encoding", "msgpack", "Default encoding to use to talk to the API")
}
