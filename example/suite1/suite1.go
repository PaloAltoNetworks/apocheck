package suite1

import (
	"context"
	"io"
	"time"

	"github.com/aporeto-inc/apocheck"
	"github.com/aporeto-inc/manipulate"
)

func init() {

	apocheck.RegisterTest(apocheck.Test{
		ID:          "apo-ns1",
		Name:        "Create a namespace and login",
		Description: "This test creates a namespace and tries to authenticate.",
		Author:      "Antoine",
		Tags:        []string{"suite1", "namespaces"},
		Function: func(ctx context.Context, w io.Writer, i apocheck.PlatformInfo, m manipulate.Manipulator) error {

			<-time.After(1 * time.Second)

			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		ID:          "apo-pu1",
		Name:        "Create a processing unit with missing attribute",
		Description: "This test creates a processing unit with attribute type missing.",
		Author:      "Antoine",
		Tags:        []string{"b", "c"},
		Function: func(ctx context.Context, w io.Writer, i apocheck.PlatformInfo, m manipulate.Manipulator) error {

			<-time.After(2 * time.Second)

			return nil
		},
	})
}
