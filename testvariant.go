package apocheck

import (
	"sort"
)

type TestVariants map[string]interface{}

func defaultTestVariant() TestVariants {
	return TestVariants{"base": nil}
}

func (v TestVariants) sorted() (out []string) {

	for k := range v {
		out = append(out, k)
	}

	sort.Strings(out)
	return out
}
