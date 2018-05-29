package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
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
			var certPoolPrivate, publicPoolPrivate *x509.CertPool
			var cert tls.Certificate

			if viper.GetString("api-public") == "" && viper.GetString("token") == "" {
				x509Cert, key, err := tglib.ReadCertificatePEM(
					viper.GetString("cert"),
					viper.GetString("key"),
					viper.GetString("key-pass"),
				)
				if err != nil {
					return err
				}

				var er error
				cert, er = tglib.ToTLSCertificate(x509Cert, key)
				if er != nil {
					return er
				}

				data, err := ioutil.ReadFile(viper.GetString("cacert-private"))
				if err != nil {
					return err
				}
				certPoolPrivate = x509.NewCertPool()
				certPoolPrivate.AppendCertsFromPEM(data)

				data, e := ioutil.ReadFile(viper.GetString("cacert-public"))
				if e != nil {
					return e
				}
				publicPoolPrivate, _ = x509.SystemCertPool()
				publicPoolPrivate.AppendCertsFromPEM(data)
			}

			ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("limit"))
			defer cancel()

			suite := mainTestSuite

			tags := viper.GetStringSlice("tag")
			ids := viper.GetStringSlice("id")
			if len(tags) > 0 {
				suite = mainTestSuite.testsWithTags(tags...)
			}
			if len(ids) > 0 {
				suite = mainTestSuite.testsWithIDs(ids...)
			}

			return newTestRunner(
				suite,
				viper.GetString("api-private"),
				certPoolPrivate,
				viper.GetString("api-public"),
				publicPoolPrivate,
				cert,
				viper.GetInt("concurrent"),
				viper.GetInt("stress"),
				viper.GetBool("verbose"),
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
	cmdRunTests.Flags().String("cacert-private", "", "Path to the private api ca certificate")
	cmdRunTests.Flags().String("cacert-public", "", "Path to the public api ca certificate")
	cmdRunTests.Flags().String("cert", "", "Path to client certificate")
	cmdRunTests.Flags().String("key-pass", "", "Password for the certificate key")
	cmdRunTests.Flags().String("key", "", "Path to client certificate key")
	cmdRunTests.Flags().String("api-private", "https://localhost:4444", "Address of the private api gateway")
	cmdRunTests.Flags().String("api-public", "https://localhost:4443", "Address of the public api gateway")
	cmdRunTests.Flags().String("token", "", "Access Token")
	cmdRunTests.Flags().String("account", "", "Account Name")
	cmdRunTests.Flags().String("config", "", "Test Configuration")
	cmdRunTests.Flags().StringSliceP("id", "i", nil, "Only run tests with the given identifier")
	cmdRunTests.Flags().StringSliceP("tag", "t", nil, "Only run tests with the given tags")

	rootCmd.AddCommand(
		versionCmd,
		cmdListTests,
		cmdRunTests,
	)

	return rootCmd
}
