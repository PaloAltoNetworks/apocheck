package apocheck

var mainTestSuite TestSuite

// RegisterTest register a test in the main suite.
func RegisterTest(t Test) { mainTestSuite[t.Name] = t }

func init() { mainTestSuite = TestSuite{} }
