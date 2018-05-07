package suite2

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/aporeto-inc/apocheck"
)

func init() {
	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a network policy and check traffic",
		Description: "This test creates a network access policy, two processing units and verifies communication between them.",
		Author:      "Antoine Mercadal",
		Tags:        []string{"suite2"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			<-time.After(time.Duration(rand.Intn(3)) * time.Second)
			if rand.Intn(10) <= 8 {
				return nil
			}

			fmt.Fprintln(t, "create a namespace")
			fmt.Fprintln(t, "add a policy")
			fmt.Fprintln(t, "send traffic")

			return fmt.Errorf("Unable to send traffic")
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Try kube-squall",
		Description: "This test tries kube-squall but we all know it always fail. It will pass at next run.",
		Author:      "Antoine Mercadal",
		Tags:        []string{"suite2"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			<-time.After(time.Duration(rand.Intn(3)) * time.Second)
			if rand.Intn(10) <= 8 {
				return nil
			}

			fmt.Fprintln(t, "start kube-squall")
			fmt.Fprintln(t, "start enforcerd")
			fmt.Fprintln(t, "send traffic")

			return fmt.Errorf("Unable to reach eventual consistency")
		},
	})
}
