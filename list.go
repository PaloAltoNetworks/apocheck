package apocheck

import "fmt"

func listTests(suites []*suiteInfo) error {

	for _, suite := range suites {
		suite.listTests()
	}

	return nil
}

func listSuites(suites []*suiteInfo) error {

	for _, suite := range suites {
		fmt.Printf("%s\n", suite)
	}

	return nil
}
