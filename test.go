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
	currentVariant string
	Variants       map[string]interface{}

	Setup    SetupFunction
	Function TestFunction
}

func (t Test) String() string {
	return fmt.Sprintf(`id         : %s
name       : %s
desc       : %s
author     : %s
categories : %s
`, t.id, t.Name, t.Description, t.Author, strings.Join(t.Tags, ", "))
}

// GetVariant retrieves current variant of the test
func (t Test) GetVariant() (string, interface{}) {

	if t.Variants == nil {
		return "", nil
	}

	return t.currentVariant, t.Variants[t.currentVariant]
}
