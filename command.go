package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.aporeto.io/tg/tglib"
)

// NewCommand generates a new CLI for regolith
func NewCommand(
	name string,
	description string,
	version string,
) *cobra.Command {

	cobra.OnInitialize(func() {
		viper.SetEnvPrefix(name)
		viper.AutomaticEnv()
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	})

	var rootCmd = &cobra.Command{
		Use:   name,
		Short: description,
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Prints the version and exit.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}

	var cmdListTests = &cobra.Command{
		Use:           "list",
		Aliases:       []string{"ls"},
		Short:         "List registered tests.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			return listTests()
		},
	}

	var cmdRunTests = &cobra.Command{
		Use:           "test",
		Aliases:       []string{"run"},
		Short:         "Run the registered tests",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			// TODO: add argument check.
			// var certPoolPrivate, certPoolPublic *x509.CertPool

			var caPoolPublic, caPoolPrivate *x509.CertPool
			var systemCert *tls.Certificate
			var err error

			if path := viper.GetString("cacert-public"); path != "" {
				caPoolPublic, err = setupPublicCA(path)
				if err != nil {
					return fmt.Errorf("unable to load public ca from path '%s': %s", path, err)
				}
			}

			if path := viper.GetString("cacert-private"); path != "" {
				caPoolPrivate, err = setupPrivateCA(path)
				if err != nil {
					return fmt.Errorf("unable to load private ca from path '%s': %s", path, err)
				}
			}

			if certPath, keyPath := viper.GetString("cert"), viper.GetString("key"); certPath != "" && keyPath != "" {
				systemCert, err = setupCerts(certPath, keyPath, viper.GetString("key-pass"))
				if err != nil {
					return err
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("limit"))
			defer cancel()

			suite := mainTestSuite

			ids := viper.GetStringSlice("id")
			if len(ids) > 0 {
				suite = mainTestSuite.testsWithIDs(viper.GetBool("verbose"), ids)
			} else {
				tags := viper.GetStringSlice("tag")
				if len(tags) > 0 {
					suite = mainTestSuite.testsWithArgs(viper.GetBool("verbose"), viper.GetBool("match-all"), tags)
				}
			}

			return newTestRunner(
				ctx,
				viper.GetString("api-private"),
				caPoolPrivate,
				systemCert,

				viper.GetString("api-public"),
				caPoolPublic,
				viper.GetString("token"),
				viper.GetString("namespace"),

				suite,
				viper.GetDuration("limit"),
				viper.GetInt("concurrent"),
				viper.GetInt("stress"),
				viper.GetBool("verbose"),
				viper.GetBool("skip-teardown"),
				viper.GetBool("stop-on-failure"),
			).Run(ctx, suite)
		},
	}

	// Parameters to connect to private API
	cmdRunTests.Flags().String("api-private", "https://127.0.0.1:4444", "Address of the private api gateway")
	cmdRunTests.Flags().String("cacert-private", os.ExpandEnv("$CERTS_FOLDER/ca-chain-system.pem"), "Path to the private api ca certificate")
	cmdRunTests.Flags().String("cert", os.ExpandEnv("$CERTS_FOLDER/system-cert.pem"), "Path to client certificate")
	cmdRunTests.Flags().String("key-pass", "", "Password for the certificate key")
	cmdRunTests.Flags().String("key", os.ExpandEnv("$CERTS_FOLDER/system-key.pem"), "Path to client certificate key")

	// Parameters to connect to public API
	cmdRunTests.Flags().String("api-public", "https://127.0.0.1:4443", "Address of the public api gateway")
	cmdRunTests.Flags().String("cacert-public", os.ExpandEnv("$CERTS_FOLDER/ca-chain-public.pem"), "Path to the public api ca certificate")
	cmdRunTests.Flags().String("token", "", "Access Token")
	cmdRunTests.Flags().String("namespace", "/", "Account Name")

	// Parameters to configure test behaviors
	cmdRunTests.Flags().BoolP("verbose", "V", false, "Show logs even on success")
	cmdRunTests.Flags().DurationP("limit", "l", 5*time.Minute, "Execution time limit.")
	cmdRunTests.Flags().IntP("concurrent", "c", 20, "Max number of concurrent tests.")
	cmdRunTests.Flags().IntP("stress", "s", 1, "Number of time to run each time in parallel.")
	cmdRunTests.Flags().StringSliceP("id", "i", nil, "Only run tests with the given identifier")
	cmdRunTests.Flags().StringSliceP("tag", "t", nil, "Only run tests with the given tags")
	cmdRunTests.Flags().BoolP("match-all", "M", false, "Match all tags specified")
	cmdRunTests.Flags().BoolP("skip-teardown", "S", false, "Skip teardown step")
	cmdRunTests.Flags().BoolP("stop-on-failure", "X", false, "Stop on the first failed test")

	rootCmd.AddCommand(
		versionCmd,
		cmdListTests,
		cmdRunTests,
	)

	return rootCmd
}
func setupPublicCA(caPublicPath string) (*x509.CertPool, error) {

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if caPublicPath != "" {
		data, err := ioutil.ReadFile(caPublicPath)
		if err != nil {
			return nil, err
		}

		pool.AppendCertsFromPEM(data)
	}

	return pool, nil
}

func setupPrivateCA(caSystemPath string) (*x509.CertPool, error) {

	data, err := ioutil.ReadFile(caSystemPath)
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
