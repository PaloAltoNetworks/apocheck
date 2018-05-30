package suite1

import (
	"context"
	"fmt"

	"github.com/aporeto-inc/apocheck"
)

func init() {

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Basic test to test apocheck steps",
		Description: "This test uses steps in setup / teardown and test functions.",
		Author:      "Satyam",
		Tags:        []string{"step"},
		Setup: func(ctx context.Context, t apocheck.TestInfo) (interface{}, apocheck.TearDownFunction, error) {
			apocheck.Step(t, "Given I have a setup step", func() error { return nil })
			return nil, func() { apocheck.Step(t, "Then the teardown step", func() error { return nil }) }, nil
		},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {
			apocheck.Step(t, "When I perform a test step", func() error { return nil })
			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Basic test to test apocheck variants",
		Description: "This test uses variants in setup / teardown and test functions.",
		Author:      "Satyam",
		Tags:        []string{"variants"},
		Variants:    apocheck.TestVariants{"no-data-variant": nil, "data-variant": "data-for-variant"},
		Setup: func(ctx context.Context, t apocheck.TestInfo) (interface{}, apocheck.TearDownFunction, error) {

			variant, data := t.TestVariant()

			setupStep := fmt.Sprintf("Given I have a setup step for variant %s with data %v", variant, data)
			apocheck.Step(t, setupStep, func() error { return nil })

			teardownStep := fmt.Sprintf("Then the teardown step for variant %s with data %v", variant, data)
			return nil, func() { apocheck.Step(t, teardownStep, func() error { return nil }) }, nil
		},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			variant, data := t.TestVariant()

			testStep := fmt.Sprintf("When I perform a test step for variant %s with data %v", variant, data)

			apocheck.Step(t, testStep, func() error { return nil })
			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a namespace and login",
		Description: "This test creates a namespace and tries to authenticate.",
		Author:      "Antoine",
		Tags:        []string{"suite1", "namespaces"},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			// <-time.After(time.Duration(rand.Intn(3)) * time.Second)
			return nil
		},
	})

	apocheck.RegisterTest(apocheck.Test{
		Name:        "Create a processing unit with missing attribute",
		Description: "This test creates a processing unit with attribute type missing.",
		Author:      "Antoine",
		Tags:        []string{"b", "c"},
		Setup: func(ctx context.Context, t apocheck.TestInfo) (interface{}, apocheck.TearDownFunction, error) {
			panic("panic!")
		},
		Function: func(ctx context.Context, t apocheck.TestInfo) error {

			// <-time.After(time.Duration(rand.Intn(3)) * time.Second)
			return nil
		},
	})
}
