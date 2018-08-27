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

	// To accept reusability of test code, we accept variants which can run the same test
	// multiple times, once for each variant with the information stored in the map.
	Variants TestVariants

	Setup    SetupFunction
	Function TestFunction
}

// MatchTags matches all tags if --match-all is set otherwise matches any tag
func (t Test) MatchTags(tags []string, matchAll bool) bool {

	m := make([]string, 0)
	for _, incoming := range tags {
		m = append(m, incoming)
	}

	if !matchAll {
		return t.matchAnyTags(m)
	}

	return t.matchAllTags(m)
}

// matchAllTags returns true if all incoming tags are matching minus exclusions
func (t Test) matchAllTags(tags []string) bool {

	if len(tags) == 0 {
		return true
	}

	for _, incoming := range tags {
		if strings.HasPrefix(incoming, "~") {
			if t.hasTag(t.Tags, strings.TrimPrefix(incoming, "~")) {
				return false
			}

			continue
		}

		if !t.hasTag(t.Tags, incoming) {
			return false
		}
	}

	return true
}

// matchAnyTags returns true if any incoming tags are matching
func (t Test) matchAnyTags(tags []string) bool {

	if len(tags) == 0 {
		return true
	}

	for _, incoming := range tags {
		if t.hasTag(t.Tags, incoming) {
			return true
		}
	}

	return false
}

// hasTag returns true if the slice has the tag
func (t Test) hasTag(tags []string, tag string) bool {
	for _, testTag := range tags {
		if tag == testTag {
			return true
		}
	}

	return false
}

// SetupMatchingVariants reduces the variants to be run to ones passed in command line
func (t Test) SetupMatchingVariants(variants []string) map[string]interface{} {

	if len(variants) == 0 {
		return nil
	}

	if t.Variants == nil {
		return defaultTestVariant()
	}

	matchingVariants := make(map[string]interface{})
	for _, v := range variants {
		if value, ok := t.Variants[v]; ok {
			matchingVariants[v] = value
		}
	}

	return matchingVariants
}

func (t Test) String() string {
	return fmt.Sprintf(`name       : %s
desc       : %s
author     : %s
categories : %s
variants   : %s
`, t.Name, t.Description, t.Author, strings.Join(t.Tags, ", "), strings.Join(t.Variants.sorted(), ", "))
}
