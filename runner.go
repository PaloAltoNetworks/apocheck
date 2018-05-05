package apocheck

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"sync"
	"time"

	"github.com/aporeto-inc/underwater/bootstrap"

	"github.com/aporeto-inc/addedeffect/apiutils"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/manipulate/maniphttp"
)

type testRun struct {
	ctx       context.Context
	test      Test
	durations []time.Duration
	loggers   []io.ReadWriter
	errs      []error
}

type testRunner struct {
	resultsChan chan testRun
	sem         chan struct{}
	api         string
	tags        []string
	stress      int
	tlsConfig   *tls.Config
	info        *bootstrap.Info
}

func newTestRunner(
	api string,
	capool *x509.CertPool,
	cert tls.Certificate,
	tags []string,
	concurrent int,
	stress int,
) *testRunner {

	return &testRunner{
		resultsChan: make(chan testRun, concurrent*stress),
		sem:         make(chan struct{}, concurrent),
		api:         api,
		tags:        tags,
		stress:      stress,
		info: &bootstrap.Info{
			BootstrapCert: cert,
			RootCAPool:    capool,
		},
		tlsConfig: &tls.Config{
			RootCAs:      capool,
			Certificates: []tls.Certificate{cert},
		},
	}
}

func (r *testRunner) execute(ctx context.Context, suite testSuite, m manipulate.Manipulator) {

	for _, test := range suite {

		select {
		case r.sem <- struct{}{}:
		case <-ctx.Done():
			break
		}

		go func(run testRun) {

			defer func() { <-r.sem }()

			var wg sync.WaitGroup
			l := &sync.Mutex{}

			for i := 0; i < r.stress; i++ {

				wg.Add(1)

				go func(iteration int) {

					defer wg.Done()

					start := time.Now()
					buf := &bytes.Buffer{}
					e := run.test.Function(ctx, buf, r.info, m, iteration)

					l.Lock()
					run.errs = append(run.errs, e)
					run.loggers = append(run.loggers, buf)
					run.durations = append(run.durations, time.Since(start))
					l.Unlock()

					r.resultsChan <- run
				}(i)
			}

			wg.Wait()

		}(testRun{
			ctx:  ctx,
			test: test,
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

	go r.execute(
		ctx,
		suite,
		maniphttp.NewHTTPManipulatorWithTLS(r.api, "", "", "", r.tlsConfig),
	)

	status := map[string]testRun{}
	var c int

	printStatus(suite, status, 0, r.stress)

	for {
		select {

		case run := <-r.resultsChan:

			c++
			status[run.test.Name] = run

			printStatus(suite, status, c, r.stress)

			if c == len(suite)*r.stress {
				printResults(status)
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
