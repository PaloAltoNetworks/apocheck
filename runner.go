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

	"github.com/aporeto-inc/addedeffect/apiutils"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/manipulate/maniphttp"
	"github.com/aporeto-inc/underwater/bootstrap"
)

type testRun struct {
	ctx       context.Context
	durations []time.Duration
	errs      []error
	loggers   []io.ReadWriter
	test      Test
	testInfo  TestInfo
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
	info             *bootstrap.Info
	resultsChan      chan testRun
	setupErrs        chan error
	status           map[string]testRun
	stress           int
	suite            testSuite
	teardowns        chan TearDownFunction
	privateTLSConfig *tls.Config
	publicTLSConfig  *tls.Config
	verbose          bool
	token            string
	account          string
	config           string
	caFilePath       string
}

func newTestRunner(
	suite testSuite,
	privateAPI string,
	privateCAPool *x509.CertPool,
	publicAPI string,
	publicCAPool *x509.CertPool,
	cert tls.Certificate,
	concurrent int,
	stress int,
	verbose bool,
	token string,
	account string,
	config string,
) *testRunner {

	return &testRunner{
		privateAPI:  privateAPI,
		publicAPI:   publicAPI,
		concurrent:  concurrent,
		resultsChan: make(chan testRun, concurrent*stress),
		setupErrs:   make(chan error),
		status:      map[string]testRun{},
		stress:      stress,
		suite:       suite,
		verbose:     verbose,
		info: &bootstrap.Info{
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

func (r *testRunner) executeIteration(ctx context.Context, test Test, m manipulate.Manipulator, data interface{}, results chan testResult) {

	sem := make(chan struct{}, r.concurrent)

	for i := 0; i < r.stress; i++ {

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func(t Test, iteration int) {

			defer func() { <-sem }()

			buf := &bytes.Buffer{}

			ti := testResult{
				test:      t,
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

			start := time.Now()
			ti.err = test.Function(ctx, TestInfo{
				testID:          test.id,
				writter:         buf,
				iteration:       iteration,
				rootManipulator: m,
				platformInfo:    r.info,
				data:            data,
				Config:          r.config,
			})

			ti.duration = time.Since(start)

		}(test, i)
	}
}

func (r *testRunner) execute(ctx context.Context, m manipulate.Manipulator) {

	sem := make(chan struct{}, r.concurrent)

	var wg sync.WaitGroup

	for _, test := range r.suite.sorted() {

		wg.Add(1)

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		variants := map[string]interface{}{"base": nil}
		if test.Variants != nil {
			variants = test.Variants
		}

		for k := range variants {
			go func(run testRun) {

				defer func() { wg.Done(); <-sem }()

				var data interface{}
				var td TearDownFunction
				hasSetup := run.test.Setup != nil

				if hasSetup {

					defer func() {
						if r := recover(); r != nil {
							printSetupError(run.test, run.testInfo, r, nil)
						}
					}()

					var err error
					data, td, err = run.test.Setup(run.ctx, run.testInfo)

					if err != nil {
						printSetupError(run.test, run.testInfo, nil, err)
						return
					}

					if td != nil {
						defer td()
					}
				}

				resultsCh := make(chan testResult)

				go r.executeIteration(ctx, run.test, m, data, resultsCh)

				var results []testResult

				for {
					select {
					case res := <-resultsCh:
						results = append(results, res)

						if len(results) == r.stress {
							printResults(run.test, run.testInfo, results, r.verbose)
							return
						}
					case <-ctx.Done():
						return
					}
				}

			}(testRun{
				ctx:  ctx,
				test: test,
				testInfo: TestInfo{
					testID:          test.id,
					variant:         k,
					rootManipulator: m,
					platformInfo:    r.info,
					Config:          r.config,
				},
			})
		}
	}

	wg.Wait()
}

func (r *testRunner) Run(ctx context.Context, suite testSuite) error {

	subctx, subCancel := context.WithTimeout(ctx, 3*time.Second)
	defer subCancel()

	var api, username, token, account string
	var tlsConfig *tls.Config

	if r.publicAPI != "" && r.token != "" {

		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		pf, err := apiutils.GetPublicCA(subctx, r.publicAPI, tlsConfig)
		if err != nil {
			return err
		}

		r.info.Platform = make(map[string]string)
		r.info.Platform["ca-public"] = string(pf)
		r.info.Platform["public-api-external"] = r.publicAPI

		api = r.publicAPI
		username = "bearer"
		token = r.token
		account = fmt.Sprintf("/%s", r.account)

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

	r.execute(ctx, maniphttp.NewHTTPManipulatorWithTLS(api, username, token, account, tlsConfig))

	return nil
}
