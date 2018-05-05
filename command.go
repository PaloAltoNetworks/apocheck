package apocheck

import (
	"context"
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

			x509Cert, key, err := tglib.ReadCertificatePEM(
				viper.GetString("cert"),
				viper.GetString("key"),
				viper.GetString("key-pass"),
			)
			if err != nil {
				return err
			}

			cert, err := tglib.ToTLSCertificate(x509Cert, key)
			if err != nil {
				return err
			}

			data, err := ioutil.ReadFile(viper.GetString("cacert"))
			if err != nil {
				return err
			}

			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM(data)

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
				viper.GetString("api"),
				certPool,
				cert,
				viper.GetStringSlice("categories"),
				viper.GetInt("concurrent"),
			).Run(ctx, suite)
		},
	}
	cmdRunTests.Flags().StringP("api", "a", "https://localhost:4443", "Address of the api gateway")
	cmdRunTests.Flags().String("cacert", "", "Path to the api ca certificate")
	cmdRunTests.Flags().String("cert", "", "Path to client certificate")
	cmdRunTests.Flags().String("key", "", "Path to client certificate key")
	cmdRunTests.Flags().String("key-pass", "", "Password for the certificate key")
	cmdRunTests.Flags().StringSliceP("tag", "t", nil, "Only run tests with the given tags")
	cmdRunTests.Flags().StringSliceP("id", "i", nil, "Only run tests with the given identifier")
	cmdRunTests.Flags().IntP("concurrent", "c", 20, "Max number of concurrent tests.")
	cmdRunTests.Flags().DurationP("limit", "l", 5*time.Minute, "Execution time limit.")

	rootCmd.AddCommand(
		versionCmd,
		cmdListTests,
		cmdRunTests,
	)

	return rootCmd
}
