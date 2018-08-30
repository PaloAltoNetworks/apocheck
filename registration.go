package apocheck

import (
	"fmt"
	"hash/fnv"
)

var mainTestSuite testSuite

// RegisterTest register a test in the main suite.
func RegisterTest(t Test) {

	if t.Name == "" {
		panic("test is missing name")
	}

	if t.Description == "" {
		panic("test is missing description")
	}

	if t.Author == "" {
		panic("test is missing author")
	}

	if t.Function == nil {
		panic("test is missing function")
	}

	if len(t.Tags) == 0 {
		panic("test is missing tags")
	}

	if t.Variants == nil {
		t.Variants = defaultTestVariant()
	}

	h := fnv.New32()
	if _, err := h.Write([]byte(t.Name + t.Description + t.Author)); err != nil {
		panic(err)
	}
	t.id = fmt.Sprintf("%x", h.Sum32())

	mainTestSuite[t.Name] = t
}

func init() { mainTestSuite = testSuite{} }
