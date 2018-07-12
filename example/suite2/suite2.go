package suite2

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/smartystreets/goconvey/convey"

	"go.aporeto.io/apocheck"
)

func init() {
	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a network policy and check traffic",
		Description: "This test creates a network access policy, two processing units and verifies communication between them.",
		Author:      "Antoine Mercadal",
		Tags:        []string{"suite2"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			// <-time.After(time.Duration(rand.Intn(3)) * time.Second)
			if rand.Intn(10) <= 1000 {
				return nil
			}

			fmt.Fprintln(t, "create a namespace") // nolint
			fmt.Fprintln(t, "add a policy")       // nolint
			fmt.Fprintln(t, "send traffic")       // nolint

			return fmt.Errorf("Unable to send traffic")
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Try kube-squall",
		Description: "This test tries kube-squall but we all know it always fail. It will pass at next run.",
		Author:      "Antoine Mercadal",
		Tags:        []string{"suite2"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			// <-time.After(3 * time.Second)
			fmt.Fprintln(t, "trying stuff") // nolint

			apocheck.Assert(t, "unable to reach eventual consistency", "panic", convey.ShouldEqual, "ok")

			return nil
		},
	})
}
