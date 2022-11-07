package apocheck

func listTests(suites []*suiteInfo) error {

	for _, suite := range suites {
		suite.listTests()
	}

	return nil
}
