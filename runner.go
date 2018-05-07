package apocheck

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
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

type testRunner struct {
	api         string
	concurrent  int
	info        *bootstrap.Info
	resultsChan chan testRun
	sem         chan struct{}
	setupErrs   chan error
	status      map[string]testRun
	stress      int
	suite       testSuite
	teardowns   chan TearDownFunction
	tlsConfig   *tls.Config
	verbose     bool
}

func newTestRunner(
	suite testSuite,
	api string,
	capool *x509.CertPool,
	cert tls.Certificate,
	concurrent int,
	stress int,
	verbose bool,
) *testRunner {

	return &testRunner{
		api:         api,
		concurrent:  concurrent,
		resultsChan: make(chan testRun, concurrent*stress),
		sem:         make(chan struct{}, concurrent),
		setupErrs:   make(chan error),
		status:      map[string]testRun{},
		stress:      stress,
		suite:       suite,
		verbose:     verbose,
		info: &bootstrap.Info{
			BootstrapCert:    cert,
			RootCAPool:       capool,
			SystemCAPool:     capool,
			SystemClientCert: cert,
		},
		tlsConfig: &tls.Config{
			RootCAs:      capool,
			Certificates: []tls.Certificate{cert},
		},
	}
}

func (r *testRunner) executeIteration(run testRun, m manipulate.Manipulator) {

	var wg sync.WaitGroup

	sem := make(chan struct{}, r.concurrent)

	for i := 0; i < r.stress; i++ {

		wg.Add(1)

		select {
		case sem <- struct{}{}:
		case <-run.ctx.Done():
			return
		}

		go func(iteration int) {

			defer func() { wg.Done(); <-sem; r.resultsChan <- run }()

			start := time.Now()
			buf := &bytes.Buffer{}

			run.testInfo.iteration = iteration
			run.testInfo.writter = buf
			run.loggers = append(run.loggers, buf)

			err := run.test.Function(run.ctx, run.testInfo)
			run.durations = append(run.durations, time.Since(start))

			if err == nil {
				run.errs = append(run.errs, nil)
				return
			}

			r := recover()

			if r == nil {
				run.errs = append(run.errs, err)
				return
			}

			if err, ok := r.(assestionError); ok {
				run.errs = append(run.errs, err)
				return
			}

			run.errs = append(run.errs, fmt.Errorf(fmt.Sprintf("Unhandled panic: %s\n%s", r, string(debug.Stack()))))
		}(i)
	}

	wg.Wait()
}

func (r *testRunner) execute(ctx context.Context, m manipulate.Manipulator) {

	for _, test := range r.suite {

		select {
		case r.sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func(run testRun) {

			var teardownFunc TearDownFunction
			var err error

			defer func() { r.teardowns <- teardownFunc; <-r.sem }()

			if run.test.Setup != nil {
				if run.testInfo.data, teardownFunc, err = run.test.Setup(run.ctx, run.testInfo); err != nil {
					r.setupErrs <- fmt.Errorf("error during setup of '%s' (%s): %s", run.test.Name, run.test.id, err)
					return
				}
			}

			r.executeIteration(run, m)

		}(testRun{
			ctx:  ctx,
			test: test,
			testInfo: TestInfo{
				testID:          test.id,
				rootManipulator: m,
				platformInfo:    r.info,
			},
		})
	}
}

func (r *testRunner) Run(ctx context.Context, suite testSuite) error {

	subctx, subCancel := context.WithTimeout(ctx, 3*time.Second)
	defer subCancel()

	pf, err := apiutils.GetConfig(subctx, r.api, r.tlsConfig)
	if err != nil {
		return err
	}

	r.info.Platform = pf
	r.teardowns = make(chan TearDownFunction, len(suite))

	go r.execute(ctx, maniphttp.NewHTTPManipulatorWithTLS(r.api, "", "", "", r.tlsConfig))

	var completed, terminated int

	printStatus(suite, r.status, 0, r.stress)

	for {
		select {

		case run := <-r.resultsChan:

			completed++

			r.status[run.test.Name] = run
			printStatus(suite, r.status, completed, r.stress)

			if r.isTerminated(suite, completed, terminated) {
				return nil
			}

		case se := <-r.setupErrs:
			fmt.Fprintln(os.Stderr, se)
			return nil

		case td := <-r.teardowns:

			terminated++

			if td != nil {
				td()
			}

			if r.isTerminated(suite, completed, terminated) {
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *testRunner) isTerminated(suite testSuite, completed, terminated int) bool {

	if terminated == len(suite) && completed == len(suite)*r.stress {
		printResults(r.status, r.verbose)
		return true
	}

	return false
}
