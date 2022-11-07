package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.aporeto.io/elemental"
	"go.aporeto.io/tg/tglib"
	"go.uber.org/zap"
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

	var cmdList = &cobra.Command{
		Use:           "list",
		Aliases:       []string{"ls"},
		Short:         "List registered tests.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
	}

	var cmdListTests = &cobra.Command{
		Use:           "tests",
		Short:         "List registered tests.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			suites := filterSuites()
			return listTests(suites)
		},
	}

	cmdListTests.Flags().StringSliceP("id", "i", nil, "Only run tests with the given identifier")
	cmdListTests.Flags().StringSliceP("tag", "t", nil, "Only run tests with the given tags")

	var cmdListSuites = &cobra.Command{
		Use:           "suites",
		Short:         "List registered suites.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			suites := filterSuites()
			return listSuites(suites)
		},
	}

	cmdListSuites.Flags().StringSliceP("suite", "Z", nil, "Only run suites specified")

	cmdList.AddCommand(cmdListTests, cmdListSuites)

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

			suites := filterSuites()

			var encoding elemental.EncodingType
			switch viper.GetString("encoding") {
			case "json":
				encoding = elemental.EncodingTypeJSON
			case "msgpack":
				encoding = elemental.EncodingTypeMSGPACK
			default:
				zap.L().Fatal("Unknown encoding type", zap.String("encoding", viper.GetString("encoding")))
			}

			for _, suite := range suites {
				err := newTestRunner(
					ctx,
					viper.GetString("build-id"),
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
					encoding,
				).Run(ctx, suite)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

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
	cmdRunTests.Flags().String("api-private", "https://127.0.0.1:4444", "Address of the private api gateway")
	cmdRunTests.Flags().String("cacert-private", defaultCaCertPrivate, "Path to the private api ca certificate")
	cmdRunTests.Flags().String("cert", defaultCert, "Path to client certificate")
	cmdRunTests.Flags().String("key-pass", "", "Password for the certificate key")
	cmdRunTests.Flags().String("key", defaultKey, "Path to client certificate key")

	// Parameters to connect to public API
	cmdRunTests.Flags().String("api-public", "https://127.0.0.1:4443", "Address of the public api gateway")
	cmdRunTests.Flags().String("cacert-public", defaultCaCertPublic, "Path to the public api ca certificate")
	cmdRunTests.Flags().String("token", "", "Access Token")
	cmdRunTests.Flags().String("namespace", "/", "Account Name")

	// Parameters to configure suite behaviors
	cmdRunTests.Flags().StringSliceP("suite", "Z", nil, "Only run suites specified")

	// Parameters to configure test behaviors
	cmdRunTests.Flags().String("encoding", "msgpack", "Default encoding to use to talk to the API")
	cmdRunTests.Flags().BoolP("verbose", "V", false, "Show logs even on success")
	cmdRunTests.Flags().DurationP("limit", "l", 20*time.Minute, "Execution time limit")
	cmdRunTests.Flags().IntP("concurrent", "c", 20, "Max number of concurrent tests")
	cmdRunTests.Flags().IntP("stress", "s", 1, "Number of time to run each time in parallel")
	cmdRunTests.Flags().StringSliceP("id", "i", nil, "Only run tests with the given identifier")
	cmdRunTests.Flags().StringSliceP("tag", "t", nil, "Only run tests with the given tags")
	cmdRunTests.Flags().BoolP("match-all", "M", false, "Match all tags specified")
	cmdRunTests.Flags().BoolP("skip-teardown", "S", false, "Skip teardown step")
	cmdRunTests.Flags().BoolP("stop-on-failure", "X", false, "Stop on the first failed test")

	rootCmd.AddCommand(
		versionCmd,
		cmdList,
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

// runSuite returns true if we should consider the suite for running
func runSuite(s *suiteInfo, names []string) bool {
	if len(names) == 0 {
		return true
	}
	for _, name := range names {
		if name == s.Name {
			return true
		}
	}
	return false
}

// filterSuites filters the suite based on ids and/or tags
func filterSuites() []*suiteInfo {
	s := []*suiteInfo{}

	names := viper.GetStringSlice("suite")
	for _, suite := range mainSuites.sorted() {

		// Filter Suites
		if !runSuite(suite, names) {
			continue
		}

		// Filter Tests in a suite
		ids := viper.GetStringSlice("id")
		if len(ids) > 0 {
			suite = suite.testsWithIDs(viper.GetBool("verbose"), ids)
		} else {
			tags := viper.GetStringSlice("tag")
			if len(tags) > 0 {
				suite = suite.testsWithArgs(viper.GetBool("verbose"), viper.GetBool("match-all"), tags)
			}
		}
		if len(suite.tests) > 0 {
			s = append(s, suite)
		}
	}
	return s
}
