package apocheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	influxdb "github.com/aporeto-inc/influxdb1-client"
	"go.aporeto.io/tg/tglib"
	"go.aporeto.io/underwater/ibatcher"
)

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

func makeInfluxClient(ctx context.Context, addr string, database string, user string, pass string, caPath string, certPath string, keyPath string, keyPass string) (ibatcher.Batcher, error) {

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

	client, err := influxdb.NewHTTPClient(
		influxdb.HTTPConfig{
			Addr:      addr,
			Username:  user,
			Password:  pass,
			TLSConfig: tlsConfig,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("unable to connect to influxdb: %s", err)
	}

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
