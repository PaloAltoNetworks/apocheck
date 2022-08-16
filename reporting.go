package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"hash/fnv"
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
	currentClient         influxdb.Client
	currentInfluxDBClient string
	currentBuildID        string
)

// InitUnitTestMetricsReporter initializes the unit tests metric reporter
// for a package. This must be at least once in the tested package in a init()
// function.
//
// It uses the following env to configure influx reporting:
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_ADDRESS: address of the influxDB to report. If empty, reporting is disabled
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_DB: database to use (default: apocheck)
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_USER: username to use to connect to influxDB
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_PASS: password associated to the username
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CA: path to the CA to trust for influxDB
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CERT: path to a client certificate to use to connect to influxDB
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CERT_KEY: path to the key associated to the client certificate
// - APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CERT_KEY_PASS: passkey to use to decrypt private key
func InitUnitTestMetricsReporter() {

	if currentClient != nil {
		return
	}

	influxAddr := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_ADDRESS")
	if influxAddr == "" {
		return
	}

	buildID := os.Getenv("APOCHECK_UNIT_TESTS_BUILD_ID")
	if buildID == "" {
		buildID = "dev"
	}

	influxDB := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_DB")
	if influxDB == "" {
		influxDB = "apocheck"
	}

	influxUser := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_USER")
	influxPass := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_PASS")
	influxTLSCAPath := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CA")
	influxTLSCertPath := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CERT")
	influxTLSCertKeyPath := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CERT_KEY")
	influxTLSCertKeyPass := os.Getenv("APOCHECK_UNIT_TESTS_METRICS_INFLUXDB_CERT_KEY_PASS")

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

	currentClient = client
	currentBuildID = buildID
	currentInfluxDBClient = influxDB
}

// Measure can be called in a unit test to measure the execution time of
// a unit test. It returns a function you need to call when the test is done.
//
// For instance:
//
//	func TestSuff(t *testing.T) {
//	    defer apocheck.Measure(t)()
//	    // test stuff
//	}
func Measure(t *testing.T) func() {

	if currentClient == nil {
		return func() {}
	}

	h := fnv.New32()
	if _, err := h.Write([]byte(t.Name())); err != nil {
		panic(err)
	}

	pkg := strings.Replace(getFrame(1).Function, fmt.Sprintf(".%s", t.Name()), "", 1)
	cfg := influxdb.BatchPointsConfig{Database: currentInfluxDBClient}
	start := time.Now()

	return func() {

		batch, err := influxdb.NewBatchPoints(cfg)
		if err != nil {
			panic(err)
		}

		batch.AddPoint(
			statsReport{
				ID:       fmt.Sprintf("%x", h.Sum32()),
				BuildID:  currentBuildID,
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
		if err := currentClient.Write(batch); err != nil {
			fmt.Fprintf(os.Stderr, "unable to post point: %s", err)
		}
	}
}

type statsReport struct {
	// 1 means OK. Everything else is an error.
	Value    int
	Duration int
	BuildID  string
	ID       string
	Name     string
	Suite    string
}

func (s statsReport) point(measurementName string) *influxdb.Point {

	pt, err := influxdb.NewPoint(
		measurementName,
		map[string]string{ // tags
			"id":    s.ID,
			"name":  s.Name,
			"suite": s.Suite,
			"build": s.BuildID,
		},
		map[string]interface{}{ // fields
			"duration": s.Duration,
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
		cadata, err := os.ReadFile(caPath)
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
