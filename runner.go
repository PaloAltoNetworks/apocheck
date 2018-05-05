package apocheck

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"time"

	"github.com/aporeto-inc/addedeffect/apiutils"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/manipulate/maniphttp"
)

type testRun struct {
	ctx     context.Context
	test    Test
	logger  io.ReadWriter
	elapsed time.Duration
	err     error
}

type testRunner struct {
	resultsChan chan testRun
	sem         chan struct{}
	api         string
	categories  []string
	tlsConfig   *tls.Config
}

func newTestRunner(
	api string,
	capool *x509.CertPool,
	cert tls.Certificate,
	categories []string,
	concurrent int,
) *testRunner {

	return &testRunner{
		resultsChan: make(chan testRun, concurrent),
		sem:         make(chan struct{}, concurrent),
		api:         api,
		categories:  categories,
		tlsConfig: &tls.Config{
			RootCAs:      capool,
			Certificates: []tls.Certificate{cert},
		},
	}
}

//suite := mainTestSuite.TestsForCategories(r.categories...)

func (r *testRunner) execute(ctx context.Context, suite TestSuite, pf PlatformInfo, m manipulate.Manipulator) {

	for _, test := range suite {

		select {
		case r.sem <- struct{}{}:
		case <-ctx.Done():
			break
		}

		go func(run testRun) {

			defer func() { <-r.sem }()

			start := time.Now()
			run.err = run.test.Function(run.ctx, run.logger, pf, m)
			run.elapsed = time.Since(start)

			r.resultsChan <- run

		}(testRun{
			ctx:    ctx,
			test:   test,
			logger: &bytes.Buffer{},
		})
	}
}

func (r *testRunner) Run(ctx context.Context, suite TestSuite) error {

	subctx, subCancel := context.WithTimeout(ctx, 3*time.Second)
	defer subCancel()

	pf, err := apiutils.GetConfig(subctx, r.api, r.tlsConfig)
	if err != nil {
		return err
	}

	go r.execute(
		ctx,
		suite,
		pf,
		maniphttp.NewHTTPManipulatorWithTLS(r.api, "", "", "", r.tlsConfig),
	)

	completed := map[string]testRun{}

	printStatus(suite, completed)

	for {
		select {

		case run := <-r.resultsChan:

			completed[run.test.Name] = run

			printStatus(suite, completed)

			if len(completed) == len(suite) {
				printResults(completed)
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
