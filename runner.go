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

func (r *testRunner) executeIteration(ctx context.Context, currTest testRun, m manipulate.Manipulator, data interface{}, results chan testResult) {

	sem := make(chan struct{}, r.concurrent)

	for i := 0; i < r.stress; i++ {

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func(t Test, iteration int) {

			buf := &bytes.Buffer{}

			defer func() { <-sem }()

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
			ti.err = currTest.test.Function(ctx, TestInfo{
				testID:          currTest.test.id,
				testVariant:     currTest.testInfo.testVariant,
				testVariantData: currTest.testInfo.testVariantData,
				writer:          buf,
				iteration:       iteration,
				rootManipulator: m,
				platformInfo:    r.info,
				data:            data,
				Config:          r.config,
				timeOfLastStep:  currTest.testInfo.timeOfLastStep,
			})

			ti.duration = time.Since(start)

		}(currTest.test, i)
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

				var data interface{}
				var td TearDownFunction
				hasSetup := run.test.Setup != nil

				if hasSetup {

					defer func() {
						if r := recover(); r != nil {
							printSetupError(run, r, nil)
						}
					}()

					var err error
					data, td, err = run.test.Setup(run.ctx, run.testInfo)
					if err != nil {
						printSetupError(run, nil, err)
						return
					}
				}

				resultsCh := make(chan testResult)

				go r.executeIteration(ctx, run, m, data, resultsCh)

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

				if r.skipTeardown {
					run.testInfo.Write([]byte("Teardown skipped.")) //nolint
				} else {
					if td != nil {
						td()
					}
				}

				fmt.Println(hdr.String())
				fmt.Println(buf.String())
			}(testRun{
				ctx:     ctx,
				test:    test,
				verbose: r.verbose,
				testInfo: TestInfo{
					testID:          test.id,
					testVariant:     variantKey,
					testVariantData: variantValue,
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

	var api, username, token, account string
	var tlsConfig *tls.Config

	if r.token != "" {

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
		username = "Bearer"
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

	if ctx.Err() != nil {
		return fmt.Errorf("Deadline exceeded. Try giving a higher time limit using -limit option (%s)", ctx.Err().Error())
	}

	return nil
}
