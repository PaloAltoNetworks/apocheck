package apocheck

import (
	"sort"
	"strings"
)

type testSuite map[string]Test

func (s testSuite) testsWithArgs(tags, variants []string) testSuite {

	ts := testSuite{}

	sort.Strings(variants)
	for _, t := range s {

		if t.MatchTags(tags) {

			t.SetupMatchingVariants(variants)

			ts[t.Name] = t
		}
	}

	return ts
}

func (s testSuite) testsWithIDs(ids ...string) testSuite {
	if len(ids) == 0 {
		return s
	}

	ts := testSuite{}

	for _, t := range s {
		for _, id := range ids {
			if id == t.id {
				ts[t.Name] = t
			}
		}
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
