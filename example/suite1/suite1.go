package suite1

import (
	"context"
	"math/rand"
	"time"

	"github.com/aporeto-inc/apocheck"
)

func init() {

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a namespace and login",
		Description: "This test creates a namespace and tries to authenticate.",
		Author:      "Antoine",
		Tags:        []string{"suite1", "namespaces"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			<-time.After(time.Duration(rand.Intn(3)) * time.Second)
			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a processing unit with missing attribute",
		Description: "This test creates a processing unit with attribute type missing.",
		Author:      "Antoine",
		Tags:        []string{"b", "c"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			<-time.After(time.Duration(rand.Intn(3)) * time.Second)
			return nil
		},
	})
}
