package apocheck

import "fmt"

func listTests(suite testSuite) error {

	for _, test := range suite.sorted() {
		fmt.Printf("%s\n", test)
	}

	return nil
}
