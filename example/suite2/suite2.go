package suite2

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/aporeto-inc/apocheck"
	"github.com/aporeto-inc/manipulate"
	"github.com/aporeto-inc/underwater/bootstrap"
)

func init() {
	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a network policy and check traffic",
		Description: "This test creates a network access policy, two processing units and verifies communication between them.",
		Author:      "Antoine Mercadal",
		Tags:        []string{"suite2"},
		Function: func(ctx context.Context, w io.Writer, i *bootstrap.Info, m manipulate.Manipulator, iteration int) error {

			<-time.After(time.Duration(rand.Intn(3)) * time.Second)
			if rand.Intn(10) <= 8 {
				return nil
			}

			fmt.Fprintln(w, "create a namespace")
			fmt.Fprintln(w, "add a policy")
			fmt.Fprintln(w, "send traffic")

			return fmt.Errorf("Unable to send traffic")
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Try kube-squall",
		Description: "This test tries kube-squall but we all know it always fail. It will pass at next run.",
		Author:      "Antoine Mercadal",
		Tags:        []string{"suite2"},
		Function: func(ctx context.Context, w io.Writer, i *bootstrap.Info, m manipulate.Manipulator, iteration int) error {

			<-time.After(time.Duration(rand.Intn(3)) * time.Second)
			if rand.Intn(10) <= 8 {
				return nil
			}

			fmt.Fprintln(w, "start kube-squall")
			fmt.Fprintln(w, "start enforcerd")
			fmt.Fprintln(w, "send traffic")

			return fmt.Errorf("Unable to reach eventual consistency")
		},
	})
}
