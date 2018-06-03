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

	"github.com/aporeto-inc/tg/tglib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			var certPoolPrivate, certPoolPublic *x509.CertPool
			var cert tls.Certificate
			var err error

			if viper.GetString("token") == "" {
				cert, certPoolPrivate, certPoolPublic, err = setupCerts()
				if err != nil {
					return err
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("limit"))
			defer cancel()

			suite := mainTestSuite

			variants := viper.GetStringSlice("variant")
			ids := viper.GetStringSlice("id")
			if len(ids) > 0 {
				suite = mainTestSuite.testsWithIDs(viper.GetBool("verbose"), ids, variants)
			} else {
				tags := viper.GetStringSlice("tag")
				if len(tags) > 0 || len(variants) > 0 {
					suite = mainTestSuite.testsWithArgs(viper.GetBool("verbose"), tags, variants)
				}
			}

			return newTestRunner(
				suite,
				viper.GetString("api-private"),
				certPoolPrivate,
				viper.GetString("api-public"),
				certPoolPublic,
				cert,
				viper.GetInt("concurrent"),
				viper.GetInt("stress"),
				viper.GetBool("verbose"),
				viper.GetBool("skip-teardown"),
				viper.GetString("token"),
				viper.GetString("account"),
				viper.GetString("config"),
			).Run(ctx, suite)
		},
	}
	cmdRunTests.Flags().BoolP("verbose", "V", false, "Show logs even on success")
	cmdRunTests.Flags().DurationP("limit", "l", 5*time.Minute, "Execution time limit.")
	cmdRunTests.Flags().IntP("concurrent", "c", 20, "Max number of concurrent tests.")
	cmdRunTests.Flags().IntP("stress", "s", 1, "Number of time to run each time in parallel.")
	cmdRunTests.Flags().String("cacert-private", os.ExpandEnv("$CERTS_FOLDER/ca-chain-system.pem"), "Path to the private api ca certificate")
	cmdRunTests.Flags().String("cacert-public", os.ExpandEnv("$CERTS_FOLDER/ca-chain-public.pem"), "Path to the public api ca certificate")
	cmdRunTests.Flags().String("cert", os.ExpandEnv("$CERTS_FOLDER/system-cert.pem"), "Path to client certificate")
	cmdRunTests.Flags().String("key-pass", "", "Password for the certificate key")
	cmdRunTests.Flags().String("key", os.ExpandEnv("$CERTS_FOLDER/system-key.pem"), "Path to client certificate key")
	cmdRunTests.Flags().String("api-private", "https://127.0.0.1:4444", "Address of the private api gateway")
	cmdRunTests.Flags().String("api-public", "https://127.0.0.1:4443", "Address of the public api gateway")
	cmdRunTests.Flags().String("token", "", "Access Token")
	cmdRunTests.Flags().String("account", "", "Account Name")
	cmdRunTests.Flags().String("config", "", "Test Configuration")
	cmdRunTests.Flags().StringSliceP("id", "i", nil, "Only run tests with the given identifier")
	cmdRunTests.Flags().StringSliceP("tag", "t", nil, "Only run tests with the given tags")
	cmdRunTests.Flags().StringSliceP("variant", "v", nil, "Only run tests with the given variants")
	cmdRunTests.Flags().BoolP("skip-teardown", "S", false, "Skip teardown step")

	rootCmd.AddCommand(
		versionCmd,
		cmdListTests,
		cmdRunTests,
	)

	return rootCmd
}

func setupCerts() (cert tls.Certificate, certPoolPrivate, certPoolPublic *x509.CertPool, err error) {

	x509Cert, key, err := tglib.ReadCertificatePEM(
		viper.GetString("cert"),
		viper.GetString("key"),
		viper.GetString("key-pass"),
	)
	if err != nil {
		return
	}

	cert, err = tglib.ToTLSCertificate(x509Cert, key)
	if err != nil {
		return
	}

	data, err := ioutil.ReadFile(viper.GetString("cacert-private"))
	if err != nil {
		return
	}
	certPoolPrivate = x509.NewCertPool()
	certPoolPrivate.AppendCertsFromPEM(data)

	data, err = ioutil.ReadFile(viper.GetString("cacert-public"))
	if err != nil {
		return
	}

	certPoolPublic, err = x509.SystemCertPool()
	if err != nil {
		return
	}

	certPoolPublic.AppendCertsFromPEM(data)

	return
}
