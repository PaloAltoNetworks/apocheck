package apocheck

import (
	"sort"
	"strings"
)

// A TestSuite represents a suite of tests.
type TestSuite map[string]Test

// TestsForCategories returns a tests matching the given categories.
func (s TestSuite) TestsForCategories(categories ...string) TestSuite {

	if len(categories) == 0 {
		return s
	}

	ts := TestSuite{}

	for _, t := range s {
		for _, c := range t.Categories {
			for _, wc := range categories {
				if wc == c {
					ts[t.Name] = t
				}
			}
		}
	}

	return ts
}

// Sorted returns the sorted suite of test by name.
func (s TestSuite) Sorted() (out []Test) {

	for _, t := range s {
		out = append(out, t)
	}

	sort.Slice(out, func(i int, j int) bool {
		return strings.Compare(out[i].Name, out[j].Name) == -1
	})

	return out
}
