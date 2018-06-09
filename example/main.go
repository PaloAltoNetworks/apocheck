package main

import (
	"go.aporeto.io/apocheck"

	// Import all the test suites
	_ "go.aporeto.io/apocheck/example/suite1"
	_ "go.aporeto.io/apocheck/example/suite2"
)

func main() {

	// Run the command.
	if err := apocheck.NewCommand("test", "this is a test", "1.0").Execute(); err != nil {
		panic(err)
	}
}
