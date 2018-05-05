package apocheck

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aporeto-inc/manipulate"
)

// A PlatformInfo contains general information about the platform.
type PlatformInfo map[string]string

// A TestFunction is the type of a function that is run by a Test.
type TestFunction func(context.Context, io.Writer, PlatformInfo, manipulate.Manipulator) error

// A Test represents an actual test.
type Test struct {
	Name        string
	Description string
	Author      string
	Categories  []string
	Function    TestFunction
}

func (t Test) String() string {
	return fmt.Sprintf(`name       : %s
desc       : %s
author     : %s
categories : %s
`, t.Name, t.Description, t.Author, strings.Join(t.Categories, ", "))
}
