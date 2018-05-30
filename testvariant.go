package apocheck

import (
	"sort"
)

type testVariants map[string]interface{}

func defaultTestVariant() testVariants {
	return testVariants{"base": nil}
}

func (v testVariants) sorted() (out []string) {

	for k := range v {
		out = append(out, k)
	}

	sort.Strings(out)
	return out
}
