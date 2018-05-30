package apocheck

import (
	"sort"
	"strings"
)

type testSuite map[string]Test

func (s testSuite) testsWithArgs(tags, variants []string) testSuite {

	if len(tags) == 0 {
		return s
	}

	ts := testSuite{}

	sort.Strings(variants)
	for _, t := range s {
		for _, c := range t.Tags {
			for _, wc := range tags {
				if wc == c {
					if len(variants) != 0 && t.Variants != nil {
						configuredVariants := t.Variants
						t.Variants = make(map[string]interface{})
						for _, v := range variants {
							if value, ok := configuredVariants[v]; ok {
								t.Variants[v] = value
							}
						}
					}

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
