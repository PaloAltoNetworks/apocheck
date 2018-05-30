package apocheck

import (
	"fmt"
	"strings"
)

// A Test represents an actual test.
type Test struct {
	id          string
	Name        string
	Description string
	Author      string
	Tags        []string

	// To allow reusability of test code, we allow variants which can run the same test
	// multiple times, once for each variant with the information stored in the map.
	Variants TestVariants

	Setup    SetupFunction
	Function TestFunction
}

func (t Test) String() string {
	return fmt.Sprintf(`id         : %s
name       : %s
desc       : %s
author     : %s
categories : %s
variants   : %s
`, t.id, t.Name, t.Description, t.Author, strings.Join(t.Tags, ", "), strings.Join(t.Variants.sorted(), ", "))
}
