package apocheck

import (
	"crypto/tls"
	"fmt"
	"io"

	"github.com/aporeto-inc/gaia/v1/golang"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/underwater/bootstrap"
)

// A TestInfo contains various information about a test.
type TestInfo struct {
	testID          string
	data            interface{}
	iteration       int
	writter         io.Writer
	rootManipulator manipulate.Manipulator
	platformInfo    *bootstrap.Info
}

// Account returns a gaia Account object that can be used for the test.
func (t TestInfo) Account(password string) *gaia.Account {

	return &gaia.Account{
		Name:     t.AccountName(),
		Password: password,
		Email:    fmt.Sprintf("user@%s.com", t.AccountName()),
		LDAPConnSecurityProtocol: gaia.AccountLDAPConnSecurityProtocolTLS,
	}
}

// TestNamespace returns a unique namespace that can be used by this test.
func (t TestInfo) TestNamespace(iteration int) string {
	return fmt.Sprintf("/%s/%s", t.AccountName(), t.testID)
}

// AccountName returns a unique account name that can be used by this test.
func (t TestInfo) AccountName() string { return fmt.Sprintf("account-%s", t.testID) }

// AccountNamespace returns the account namespace that can be used by this test.
func (t TestInfo) AccountNamespace() string { return fmt.Sprintf("/account-%s", t.testID) }

// PublicAPI returns the public api url.
func (t TestInfo) PublicAPI() string { return t.platformInfo.Platform["public-api-external"] }

// PrivateAPI returns the private api url.
func (t TestInfo) PrivateAPI() string { return t.platformInfo.Platform["private-api-external"] }

// SetupInfo returns the eventual object stored by the Setup function.
func (t TestInfo) SetupInfo() interface{} { return t.data }

// PublicTLSConfig returns a tls config that can be used to connect to public API.
func (t TestInfo) PublicTLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs: t.platformInfo.RootCAPool,
	}
}

// PrivateTLSConfig returns a tls config that can be used to connect to private API.
func (t TestInfo) PrivateTLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs: t.platformInfo.SystemCAPool,
	}
}

// Iteration returns the test iteration number.
func (t TestInfo) Iteration() int { return t.iteration }

// TestID returns the test ID
func (t TestInfo) TestID() string { return t.testID }

// RootManipulator returns the root manipulator.
func (t TestInfo) RootManipulator() manipulate.Manipulator { return t.rootManipulator }

// PlatformInfo returns the platform information.
func (t TestInfo) PlatformInfo() *bootstrap.Info { return t.platformInfo }

func (t TestInfo) Write(p []byte) (n int, err error) { return t.writter.Write(p) }
