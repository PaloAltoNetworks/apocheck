package apocheck

import "fmt"

func listTests() error {

	for _, test := range mainTestSuite.sorted() {
		fmt.Printf("%s\n", test)
	}

	return nil
}
