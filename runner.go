package apocheck

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"go.aporeto.io/addedeffect/apiutils"
	"go.aporeto.io/gaia"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
	"go.aporeto.io/midgard-lib/client"
	"go.aporeto.io/underwater/platform"
)

type testRun struct {
	ctx      context.Context
	test     Test
	testInfo TestInfo
	verbose  bool
}

type testResult struct {
	err       error
	reader    io.Reader
	duration  time.Duration
	test      Test
	iteration int
	stack     []byte
}

type testRunner struct {
	privateAPI       string
	publicAPI        string
	concurrent       int
	info             *platform.Info
	resultsChan      chan testRun
	setupErrs        chan error
	status           map[string]testRun
	stress           int
	suite            testSuite
	teardowns        chan TearDownFunction
	timeout          time.Duration
	privateTLSConfig *tls.Config
	publicTLSConfig  *tls.Config
	verbose          bool
	skipTeardown     bool
	stopOnFailure    bool
	token            string
	account          string
	config           string
	appCreds         []byte
}

func newTestRunner(
	suite testSuite,
	privateAPI string,
	privateCAPool *x509.CertPool,
	publicAPI string,
	publicCAPool *x509.CertPool,
	cert tls.Certificate,
	timeout time.Duration,
	concurrent int,
	stress int,
	verbose bool,
	skipTeardown bool,
	stopOnFailure bool,
	token string,
	account string,
	config string,
	appCreds []byte,
) *testRunner {

	return &testRunner{
		privateAPI:    privateAPI,
		publicAPI:     publicAPI,
		concurrent:    concurrent,
		resultsChan:   make(chan testRun, concurrent*stress),
		setupErrs:     make(chan error),
		status:        map[string]testRun{},
		stress:        stress,
		suite:         suite,
		timeout:       timeout,
		verbose:       verbose,
		skipTeardown:  skipTeardown,
		stopOnFailure: stopOnFailure,
		info: &platform.Info{
			BootstrapCert:    cert,
			RootCAPool:       publicCAPool,
			SystemCAPool:     privateCAPool,
			SystemClientCert: cert,
		},
		publicTLSConfig: &tls.Config{
			RootCAs: publicCAPool,
		},
		privateTLSConfig: &tls.Config{
			RootCAs:      privateCAPool,
			Certificates: []tls.Certificate{cert},
		},
		token:    token,
		account:  account,
		config:   config,
		appCreds: appCreds,
	}
}

func (r *testRunner) executeIteration(ctx context.Context, currTest testRun, m manipulate.Manipulator, results chan testResult) {
	sem := make(chan struct{}, r.concurrent)

	for i := 0; i < r.stress; i++ {

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func(t testRun, iteration int) {
			var data interface{}
			var td TearDownFunction
			var err error

			buf := &bytes.Buffer{}

			defer func() { <-sem }()

			ti := testResult{
				test:      t.test,
				reader:    buf,
				iteration: iteration,
			}

			defer func() {

				defer func() { results <- ti }()

				// recover remote code.
				r := recover()
				if r == nil {
					return
				}

				err, ok := r.(assestionError)
				if ok {
					ti.err = err
					return
				}

				ti.err = fmt.Errorf("Unhandled panic: %s", r)
				ti.stack = debug.Stack()
			}()

			subTestInfo := TestInfo{
				testID:          NewUUID(),
				writer:          buf,
				iteration:       iteration,
				timeout:         r.timeout,
				rootManipulator: m,
				platformInfo:    r.info,
				data:            data,
				Config:          r.config,
				timeOfLastStep:  t.testInfo.timeOfLastStep,
			}

			if t.test.Setup != nil {
				data, td, err = t.test.Setup(t.ctx, subTestInfo)
				if err != nil {
					printSetupError(t, nil, err)
					return
				}
				subTestInfo.data = data

				defer func() {
					if r.skipTeardown {
						subTestInfo.Write([]byte("Teardown skipped.")) //nolint
					} else if td != nil {
						td()
					}
				}()
			}

			start := time.Now()
			ti.err = t.test.Function(ctx, subTestInfo)
			ti.duration = time.Since(start)

		}(currTest, i)
	}
}

func (r *testRunner) execute(ctx context.Context, m manipulate.Manipulator) error {
	sem := make(chan struct{}, r.concurrent)
	done := make(chan struct{})
	stop := make(chan struct{})

	var wg sync.WaitGroup
	var err error

	for _, test := range r.suite.sorted() {

		wg.Add(1)

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return err
		case <-stop:
			break
		}

		go func(run testRun) {

			buf := &bytes.Buffer{}
			hdr := &bytes.Buffer{}

			run.testInfo.writer = buf
			run.testInfo.header = hdr

			defer func() { wg.Done(); <-sem }()

			resultsCh := make(chan testResult)

			go r.executeIteration(ctx, run, m, resultsCh)

			var results []testResult

			for b := true; b; {
				select {
				case res := <-resultsCh:
					results = append(results, res)

					if res.err != nil {
						err = res.err

						if r.stopOnFailure {
							appendResults(run, results, r.verbose)
							fmt.Println(hdr.String())
							fmt.Println(buf.String())
							close(stop)

							return
						}
					}

					if len(results) == r.stress {
						appendResults(run, results, r.verbose)
						b = false
					}
				case <-ctx.Done():
					b = false
				}
			}

			if hdr.String() != "" {
				fmt.Println(hdr.String())
			}
			if buf.String() != "" {
				fmt.Println(buf.String())
			}
		}(testRun{
			ctx:     ctx,
			test:    test,
			verbose: r.verbose,
			testInfo: TestInfo{
				timeout:         r.timeout,
				rootManipulator: m,
				platformInfo:    r.info,
				Config:          r.config,
				timeOfLastStep:  time.Now(),
			},
		})
	}

	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
	case <-stop:
	}

	return err
}

func (r *testRunner) Run(ctx context.Context, suite testSuite) error {

	subctx, subCancel := context.WithTimeout(ctx, 3*time.Second)
	defer subCancel()

	var api, username, token, namespace string
	var tlsConfig *tls.Config

	if r.appCreds != nil {
		var err error
		var creds *gaia.Credential

		creds, tlsConfig, err = midgardclient.ParseCredentials(r.appCreds)
		if err != nil {
			return err
		}

		api = r.publicAPI
		username = r.account
		namespace = fmt.Sprintf("/%s", r.account)

		mclient := midgardclient.NewClientWithTLS(api, tlsConfig)
		token, err = mclient.IssueFromCertificate(context.TODO(), 5*time.Hour)
		if err != nil {
			return err
		}

		ca, err := base64.StdEncoding.DecodeString(creds.CertificateAuthority)
		if err != nil {
			return err
		}

		r.info.Platform = make(map[string]string)
		r.info.Platform["ca-public"] = string(ca)
		r.info.Platform["public-api-external"] = r.publicAPI
	} else if r.token != "" || r.privateAPI == "" {
		// We want the integration tests to be able to run on our preprod/prod platform
		// These platforms don't and can not expose the private API,
		// In that case, we recreate a Platform Info structure
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true, // nolint
		}

		certAuthority, err := apiutils.GetPublicCA(subctx, r.publicAPI, tlsConfig)
		if err != nil {
			return err
		}

		r.info.Platform = make(map[string]string)
		r.info.Platform["ca-public"] = string(certAuthority)
		r.info.Platform["public-api-external"] = r.publicAPI

		api = r.publicAPI
		namespace = fmt.Sprintf("/%s", r.account)
		if r.token != "" {
			username = "Bearer"
			token = r.token
		} else {
			tlsConfig.RootCAs = r.info.RootCAPool
			tlsConfig.Certificates = []tls.Certificate{r.info.SystemClientCert}
		}
	} else if r.privateAPI != "" {
		// In that case, we assume that platform is in under our control and can be open.
		// We want to set InsecureSkipVerify to support deployments like docker
		// swarm where we only have the IP address
		r.privateTLSConfig.InsecureSkipVerify = true

		pf, err := apiutils.GetConfig(subctx, r.privateAPI, r.privateTLSConfig)
		if err != nil {
			return err
		}

		api = r.privateAPI
		tlsConfig = r.privateTLSConfig

		r.info.Platform = pf

		// In case of private API, we need to be able to inject
		// a new internal api to access the private APIs exposed by private Services.
		r.info.Platform["public-api-internal"] = r.privateAPI
	}

	m, err := maniphttp.New(
		context.Background(),
		api,
		maniphttp.OptionCredentials(username, token),
		maniphttp.OptionNamespace(namespace),
		maniphttp.OptionTLSConfig(tlsConfig))
	if err != nil {
		return err
	}

	r.teardowns = make(chan TearDownFunction, len(suite))
	if err := r.execute(ctx, m); err != nil {
		return fmt.Errorf("Failed test(s). Please check logs")
	}

	if ctx.Err() != nil {
		return fmt.Errorf("Deadline exceeded. Try giving a higher time limit using --limit option (%s)", ctx.Err())
	}

	return nil
}
