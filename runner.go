package apocheck

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"go.aporeto.io/addedeffect/apiutils"
	"go.aporeto.io/manipulate"
	"go.aporeto.io/manipulate/maniphttp"
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
	token            string
	account          string
	config           string
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
	token string,
	account string,
	config string,
) *testRunner {

	return &testRunner{
		privateAPI:   privateAPI,
		publicAPI:    publicAPI,
		concurrent:   concurrent,
		resultsChan:  make(chan testRun, concurrent*stress),
		setupErrs:    make(chan error),
		status:       map[string]testRun{},
		stress:       stress,
		suite:        suite,
		timeout:      timeout,
		verbose:      verbose,
		skipTeardown: skipTeardown,
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
		token:   token,
		account: account,
		config:  config,
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

			t.test.id = NewUUID()
			t.testInfo.testID = t.test.id

			if t.test.Setup != nil {
				data, td, err = t.test.Setup(t.ctx, t.testInfo)
				if err != nil {
					printSetupError(t, nil, err)
					return
				}

				defer func() {
					if r.skipTeardown {
						t.testInfo.Write([]byte("Teardown skipped.")) //nolint
					} else if td != nil {
						td()
					}
				}()
			}

			start := time.Now()
			ti.err = t.test.Function(ctx, TestInfo{
				testID:          t.test.id,
				testVariant:     t.testInfo.testVariant,
				testVariantData: t.testInfo.testVariantData,
				writer:          buf,
				iteration:       iteration,
				timeout:         r.timeout,
				rootManipulator: m,
				platformInfo:    r.info,
				data:            data,
				Config:          r.config,
				timeOfLastStep:  t.testInfo.timeOfLastStep,
			})

			ti.duration = time.Since(start)

		}(currTest, i)
	}
}

func (r *testRunner) execute(ctx context.Context, m manipulate.Manipulator) {

	sem := make(chan struct{}, r.concurrent)

	var wg sync.WaitGroup

	for _, test := range r.suite.sorted() {

		for _, variantKey := range test.Variants.sorted() {

			wg.Add(1)

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}

			variantValue := test.Variants[variantKey]

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
						if len(results) == r.stress {
							appendResults(run, results, r.verbose)
							b = false
						}
					case <-ctx.Done():
						b = false
					}
				}

				fmt.Println(hdr.String())
				fmt.Println(buf.String())
			}(testRun{
				ctx:     ctx,
				test:    test,
				verbose: r.verbose,
				testInfo: TestInfo{
					testVariant:     variantKey,
					testVariantData: variantValue,
					timeout:         r.timeout,
					rootManipulator: m,
					platformInfo:    r.info,
					Config:          r.config,
					timeOfLastStep:  time.Now(),
				},
			})
		}
	}

	wg.Wait()
}

func (r *testRunner) Run(ctx context.Context, suite testSuite) error {

	subctx, subCancel := context.WithTimeout(ctx, 3*time.Second)
	defer subCancel()

	var api, username, token, namespace string
	var tlsConfig *tls.Config

	if r.privateAPI == "" || r.token != "" {

		tlsConfig = &tls.Config{
			InsecureSkipVerify: true, // nolint
		}

		pf, err := apiutils.GetPublicCA(subctx, r.publicAPI, tlsConfig)
		if err != nil {
			return err
		}

		r.info.Platform = make(map[string]string)
		r.info.Platform["ca-public"] = string(pf)
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

	} else {

		pf, err := apiutils.GetConfig(subctx, r.privateAPI, r.privateTLSConfig)
		if err != nil {
			return err
		}

		api = r.privateAPI
		tlsConfig = r.privateTLSConfig

		r.info.Platform = pf
	}

	r.teardowns = make(chan TearDownFunction, len(suite))

	r.execute(ctx, maniphttp.NewHTTPManipulatorWithTLS(api, username, token, namespace, tlsConfig))

	if ctx.Err() != nil {
		return fmt.Errorf("Deadline exceeded. Try giving a higher time limit using --limit option (%s)", ctx.Err().Error())
	}

	return nil
}
