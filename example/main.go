package main

import (
	"github.com/aporeto-inc/apocheck"

	// Import all the test suites
	_ "github.com/aporeto-inc/apocheck/example/suite1"
	_ "github.com/aporeto-inc/apocheck/example/suite2"
)

func main() {

	// Run the command.
	if err := apocheck.NewCommand("test", "this is a test", "1.0").Execute(); err != nil {
		panic(err)
	}
}
