package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aporeto-inc/gaia/v1/golang"

	"github.com/aporeto-inc/apocheck"
	"github.com/aporeto-inc/manipulate"
)

func main() {

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Test 1",
		Description: "Super test",
		Author:      "Antoine",
		Categories:  []string{"a", "b"},
		Function: func(ctx context.Context, w io.Writer, i apocheck.PlatformInfo, m manipulate.Manipulator) error {

			<-time.After(1 * time.Second)
			dst := gaia.NamespacesList{}
			mctx := manipulate.NewContext()
			mctx.Namespace = "/"

			if err := m.RetrieveMany(mctx, &dst); err != nil {
				return err
			}

			fmt.Fprintln(w, dst)

			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Test 2",
		Description: "Super test but deux",
		Author:      "Antoine",
		Categories:  []string{"b", "c"},
		Function: func(ctx context.Context, w io.Writer, i apocheck.PlatformInfo, m manipulate.Manipulator) error {
			<-time.After(2 * time.Second)
			fmt.Fprintln(w, "coucou")
			// return fmt.Errorf("NOOOOOOOOOOO")
			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Test 3",
		Description: "Super test but deux",
		Author:      "Antoine",
		Categories:  []string{"b", "c"},
		Function: func(ctx context.Context, w io.Writer, i apocheck.PlatformInfo, m manipulate.Manipulator) error {
			<-time.After(3 * time.Second)

			fmt.Fprintln(w, "create a namespace")
			fmt.Fprintln(w, "add a policy")
			fmt.Fprintln(w, "send traffic")

			return fmt.Errorf("Unable to do things")
			// return nil
		},
	})

	if err := apocheck.NewCommand("test", "this is a test", "1.0").Execute(); err != nil {
		panic(err)
	}
}
