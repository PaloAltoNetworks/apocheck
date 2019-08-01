package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	influxdb "github.com/aporeto-inc/influxdb1-client"
	"go.aporeto.io/tg/tglib"
	"go.aporeto.io/underwater/ibatcher"
)

var (
	defaultClient influxdb.Client
	defaultDBName string
)

// InitUnitTestMetricsReporter initializes the unit tests metric reporter
// for a package. This must be at least once in the tested package in a init()
// function.
//
// This will install the following flags to go test command:
//  -apocheck.metrics.influxdb-address: address of the influxDB to report. If empty, reporting is disabled
//  -apocheck.metrics.influxdb-db: database to use (default: apocheck)
//  -apocheck.metrics.influxdb-user: username to use to connect to influxDB
//  -apocheck.metrics.influxdb-pass: password associated to the username
//  -apocheck.metrics.influxdb-tls-ca: path to the CA to trust for influxDB
//  -apocheck.metrics.influxdb-tls-cert: path to a client certificate to use to connect to influxDB
//  -apocheck.metrics.influxdb-tls-cert-key: path to the key associated to the client certificate
//  -apocheck.metrics.influxdb-tls-cert-key-pass: passkey to use to decrypt private key
func InitUnitTestMetricsReporter() {

	if defaultClient != nil {
		return
	}

	var influxAddr string
	var influxDB string
	var influxUser string
	var influxPass string
	var influxTLSCAPath string
	var influxTLSCertPath string
	var influxTLSCertKeyPath string
	var influxTLSCertKeyPass string

	flag.StringVar(&influxAddr, "apocheck.metrics.influxdb-address", "", "If set, reports test metrics to influxb")
	flag.StringVar(&influxDB, "apocheck.metrics.influxdb-db", "apocheck", "Database name")
	flag.StringVar(&influxUser, "apocheck.metrics.influxdb-user", "admin", "InfluxDB username")
	flag.StringVar(&influxPass, "apocheck.metrics.influxdb-pass", "aporeto", "InfluxDB password")
	flag.StringVar(&influxTLSCAPath, "apocheck.metrics.influxdb-tls-ca", "", "Path to the CA")
	flag.StringVar(&influxTLSCertPath, "apocheck.metrics.influxdb-tls-cert", "", "Path to the client certificate")
	flag.StringVar(&influxTLSCertKeyPath, "apocheck.metrics.influxdb-tls-cert-key", "", "Path to the client certificate's key")
	flag.StringVar(&influxTLSCertKeyPass, "apocheck.metrics.influxdb-tls-cert-key-pass", "", "Passkey for the client certificate's key")

	flag.Parse()

	if influxAddr == "" {
		return
	}

	client, err := makeInfluxDBClient(
		influxAddr,
		influxUser,
		influxPass,
		influxTLSCAPath,
		influxTLSCertPath,
		influxTLSCertKeyPath,
		influxTLSCertKeyPass,
	)
	if err != nil {
		panic(fmt.Sprintf("unable to initialize influxdb client: %s", err))
	}

	defaultClient = client
	defaultDBName = influxDB
}

// Measure can be called in a unit test to measure the execution time of
// a unit test. It returns a function you need to call when the test is done.
//
// For instance:
//      func TestSuff(t *testing.T) {
//          defer apocheck.Measure(t)()
//          // test stuff
//      }
func Measure(t *testing.T) func() {

	if defaultClient == nil {
		return func() {}
	}

	h := fnv.New32()
	if _, err := h.Write([]byte(t.Name())); err != nil {
		panic(err)
	}

	pkg := strings.Replace(getFrame(1).Function, fmt.Sprintf(".%s", t.Name()), "", 1)
	cfg := influxdb.BatchPointsConfig{Database: defaultDBName}
	start := time.Now()

	return func() {

		batch, err := influxdb.NewBatchPoints(cfg)
		if err != nil {
			panic(err)
		}

		batch.AddPoint(
			statsReport{
				ID:       fmt.Sprintf("%x", h.Sum32()),
				Suite:    pkg,
				Name:     t.Name(),
				Duration: int(time.Since(start)),
				Value: func() int {
					if t.Failed() {
						return 0
					}
					return 1
				}(),
			}.point("apocheck_unit_tests"),
		)
		if err := defaultClient.Write(batch); err != nil {
			fmt.Fprintf(os.Stderr, "unable to post point: %s", err)
		}
	}
}

type statsReport struct {
	// 1 means OK. Everything else is an error.
	Value    int
	Duration int
	ID       string
	Name     string
	Suite    string
	Message  string
}

func (s statsReport) point(measurementName string) *influxdb.Point {

	pt, err := influxdb.NewPoint(
		measurementName,
		map[string]string{ // tags
			"id":    s.ID,
			"name":  s.Name,
			"suite": s.Suite,
		},
		map[string]interface{}{ // fields
			"duration": s.Duration,
			"message":  s.Message,
			"value":    s.Value,
		},
	)

	if err != nil {
		panic("unable to build influxdb.Point")
	}

	return pt
}

func makeInfluxDBClient(addr string, user string, pass string, caPath string, certPath string, keyPath string, keyPass string) (influxdb.Client, error) {

	tlsConfig := &tls.Config{}

	if caPath != "" {
		cadata, err := ioutil.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read ca: %s", err)
		}

		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(cadata)

	} else {
		var err error
		tlsConfig.RootCAs, err = x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("unable to open system cert pool: %s", err)
		}
	}

	if certPath != "" {
		cert, key, err := tglib.ReadCertificatePEM(certPath, keyPath, keyPass)
		if err != nil {
			return nil, fmt.Errorf("unable to load client certificate: %s", err)
		}

		tlsCert, err := tglib.ToTLSCertificate(cert, key)
		if err != nil {
			return nil, fmt.Errorf("unable to build tls certificate: %s", err)
		}

		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	return influxdb.NewHTTPClient(
		influxdb.HTTPConfig{
			Addr:      addr,
			Username:  user,
			Password:  pass,
			TLSConfig: tlsConfig,
		},
	)
}

func makeInfluxDBBatcher(ctx context.Context, client influxdb.Client, database string) (ibatcher.Batcher, error) {

	errCh := make(chan error)

	go func() {
		for {
			select {
			case e := <-errCh:
				fmt.Fprintf(os.Stderr, "metrics error: %s\n", e)
			case <-ctx.Done():
				return
			}
		}
	}()

	return ibatcher.New(client, database, ibatcher.OptionErrChan(errCh)), nil
}

func getFrame(skipFrames int) runtime.Frame {

	targetFrameIndex := skipFrames + 2
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n <= 0 {
		return frame
	}

	frames := runtime.CallersFrames(programCounters[:n])
	for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
		var frameCandidate runtime.Frame
		frameCandidate, more = frames.Next()
		if frameIndex == targetFrameIndex {
			frame = frameCandidate
		}
	}

	return frame
}
