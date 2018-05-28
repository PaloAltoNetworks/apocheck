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
	Variant     string
	Variants    map[string]interface{}

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

// GetVariants retrieves variants of the test
func (t Test) GetVariants() (string, interface{}) {

	return t.Variant, t.Variants[t.Variant]
}
