package apocheck

var mainTestSuite testSuite

// RegisterTest register a test in the main suite.
func RegisterTest(t Test) { mainTestSuite[t.Name] = t }

func init() { mainTestSuite = testSuite{} }
