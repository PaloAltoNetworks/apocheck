package apocheck

import (
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"go.aporeto.io/elemental"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/underwater/platform"
)

// A TestInfo contains various information about a test.
type TestInfo struct {
	data              interface{}
	header            io.Writer
	iteration         int
	platformInfo      *platform.Info
	privateAPI        string
	privateTLSConfig  *tls.Config
	publicAPI         string
	publicManipulator manipulate.Manipulator
	publicTLSConfig   *tls.Config
	rootManipulator   manipulate.Manipulator
	testID            string
	timeOfLastStep    time.Time
	timeout           time.Duration
	writer            io.Writer
	encoding          elemental.EncodingType
}

// Account returns a gaia Account object that can be used for the test.
func (t TestInfo) Account(password string) *gaia.Account {

	// nolint
	return &gaia.Account{
		Name:     t.AccountName(),
		Password: password,
		Email:    fmt.Sprintf("user@%s.com", t.AccountName()),
	}
}

// TestNamespace returns a unique namespace that can be used by this test.
func (t TestInfo) TestNamespace(iteration int) string {
	return fmt.Sprintf("/%s/%s", t.AccountName(), t.testID)
}

// AccountName returns a unique account name that can be used by this test.
func (t TestInfo) AccountName() string {
	return fmt.Sprintf("account-%s", t.testID)
}

// AccountNamespace returns the account namespace that can be used by this test.
func (t TestInfo) AccountNamespace() string {
	return fmt.Sprintf("/account-%s", t.testID)
}

// SetupInfo returns the eventual object stored by the Setup function.
func (t TestInfo) SetupInfo() interface{} {
	return t.data
}

// Iteration returns the test iteration number.
func (t TestInfo) Iteration() int {
	return t.iteration
}

// TestID returns the test ID
func (t TestInfo) TestID() string {
	return t.testID
}

// RootManipulator returns the root manipulator if any.
func (t TestInfo) RootManipulator() manipulate.Manipulator {
	return t.rootManipulator
}

// PublicManipulator returns the public manipulator if any.
func (t TestInfo) PublicManipulator() manipulate.Manipulator {
	return t.publicManipulator
}

// PublicAPI returns the public API endpoint.
func (t TestInfo) PublicAPI() string {
	return t.publicAPI
}

// PrivateAPI returns the private API endpoint.
func (t TestInfo) PrivateAPI() string {
	return t.privateAPI
}

// PublicTLSConfig returns the public TLS config.
func (t TestInfo) PublicTLSConfig() *tls.Config {
	return t.publicTLSConfig
}

// PrivateTLSConfig returns the public TLS config.
func (t TestInfo) PrivateTLSConfig() *tls.Config {
	return t.privateTLSConfig
}

// WriteHeader performs a write at the header
func (t TestInfo) WriteHeader(p []byte) (n int, err error) {
	return t.header.Write(p)
}

// Write performs a write
func (t TestInfo) Write(p []byte) (n int, err error) {
	return t.writer.Write(p)
}

// TimeSinceLastStep provides the time since last step or assertion
func (t TestInfo) TimeSinceLastStep() string {
	d := time.Since(t.timeOfLastStep)
	return d.Round(time.Millisecond).String()
}

// Timeout provides the duration before the test timeout.
func (t TestInfo) Timeout() time.Duration {
	return t.timeout
}
