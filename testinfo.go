package apocheck

import (
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/underwater/platform"
)

// A TestInfo contains various information about a test.
type TestInfo struct {
	testID          string
	testVariant     string
	testVariantData interface{}
	data            interface{}
	iteration       int
	header          io.Writer
	writer          io.Writer
	rootManipulator manipulate.Manipulator
	platformInfo    *platform.Info
	Config          string
	timeOfLastStep  time.Time
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

// TestVariant returns the current test variant and data
func (t TestInfo) TestVariant() (string, interface{}) { return t.testVariant, t.testVariantData }

// RootManipulator returns the root manipulator.
func (t TestInfo) RootManipulator() manipulate.Manipulator { return t.rootManipulator }

// PlatformInfo returns the platform information.
func (t TestInfo) PlatformInfo() *platform.Info { return t.platformInfo }

// WriteHeader performs a write at the header
func (t TestInfo) WriteHeader(p []byte) (n int, err error) { return t.header.Write(p) }

// Write performs a write
func (t TestInfo) Write(p []byte) (n int, err error) { return t.writer.Write(p) }

// TimeSinceLastStep provides the time since last step or assertion
func (t TestInfo) TimeSinceLastStep() string {
	d := time.Since(t.timeOfLastStep)
	return d.Round(time.Millisecond).String()
}
