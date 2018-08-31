package apocheck

import (
	"fmt"
	"sort"
	"strings"
)

type testSuite map[string]Test

func (s testSuite) testsWithArgs(verbose, matchAll bool, tags []string) testSuite {

	ts := testSuite{}

	if verbose {
		fmt.Println("Running Tests:")
	}

	for _, t := range s {

		if !t.MatchTags(tags, matchAll) {
			continue
		}

		if verbose {
			fmt.Println(" - " + t.Name)
		}

		ts[t.Name] = t
	}

	if verbose && len(ts) == 0 {
		fmt.Println("No matching tests found.")
	}

	return ts
}

func (s testSuite) testsWithIDs(verbose bool, ids []string) testSuite {
	if len(ids) == 0 {
		return s
	}

	ts := testSuite{}

	if verbose {
		fmt.Println("Running Tests:")
	}

	for _, t := range s {
		for _, id := range ids {
			if id == t.id {

				if verbose {
					fmt.Println(" - " + t.Name)
				}

				if verbose {
					fmt.Println(" - " + t.Name)
				}

				ts[t.Name] = t
			}
		}
	}

	if verbose && len(ts) == 0 {
		fmt.Println("No matching tests found.")
	}

	return ts
}

func (s testSuite) sorted() (out []Test) {

	for _, t := range s {
		out = append(out, t)
	}

	sort.Slice(out, func(i int, j int) bool {
		return strings.Compare(out[i].Name, out[j].Name) == -1
	})

	return out
}
