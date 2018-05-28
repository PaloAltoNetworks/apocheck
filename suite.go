package apocheck

import (
	"sort"
	"strings"
)

type testSuite map[string]Test

func (s testSuite) testsWithTags(tags ...string) testSuite {

	if len(tags) == 0 {
		return s
	}

	ts := testSuite{}

	for _, t := range s {
		for _, c := range t.Tags {
			for _, wc := range tags {
				if wc == c {
					ts[t.Name] = t
				}
			}
		}
	}

	return ts
}

func (s testSuite) testsWithVariants(variants ...string) testSuite {

	if len(variants) == 0 {
		return s
	}

	ts := testSuite{}

	for _, t := range s {
		for _, c := range t.Variants {
			for _, wc := range variants {
				if wc == c {
					ts[t.Name] = t
				}
			}
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
