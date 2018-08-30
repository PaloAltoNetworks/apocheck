package apocheck

import (
	"fmt"
	"io/ioutil"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/buger/goterm"
	wordwrap "github.com/mitchellh/go-wordwrap"
)

var printLock = &sync.Mutex{}

func printSetupError(curTest testRun, recovery interface{}, err error) {

	printLock.Lock()
	defer printLock.Unlock()

	fmt.Println()
	fmt.Printf("%s\n",
		goterm.Bold(
			goterm.Color(
				fmt.Sprintf("%s FAIL %s (variant %s)",
					curTest.test.id,
					curTest.test.Name,
					curTest.testInfo.testVariant,
				),
				goterm.YELLOW,
			),
		),
	)

	fmt.Println()
	fmt.Println(goterm.Color("setup function:", goterm.MAGENTA))
	fmt.Println()

	if recovery != nil {
		fmt.Println("panic:", recovery)
		fmt.Println(string(debug.Stack()))
	}

	if err != nil {
		fmt.Println(goterm.Color(fmt.Sprintf("  error: %s", err), goterm.RED))
	}

	fmt.Println()
}

func createHeader(currTest testRun, results []testResult, showOnSuccess bool) (failed bool) {

	failed = hasErrors(results)

	resultString := "FAIL"
	if !failed {
		resultString = "PASS"
	}

	if !failed && !showOnSuccess {
		output := fmt.Sprintf("%s\n",
			goterm.Color(
				fmt.Sprintf("%s : %s (variant %s) %s",
					resultString,
					currTest.test.Name,
					currTest.testInfo.testVariant,
					goterm.Color(fmt.Sprintf("it: %d, avg: %s", len(results), averageTime(results)), goterm.BLUE),
				),
				goterm.GREEN,
			))
		currTest.testInfo.WriteHeader([]byte(output)) // nolint
		return
	}

	color := goterm.GREEN
	if failed {
		color = goterm.YELLOW
	}

	output := fmt.Sprintf("\n%s\n%s\n",
		goterm.Bold(
			goterm.Color(
				fmt.Sprintf("%s : %s (variant %s)",
					resultString,
					currTest.test.Name,
					currTest.testInfo.testVariant,
				),
				color),
		),
		wordwrap.WrapString(fmt.Sprintf("  %s — %s", currTest.test.Description, currTest.test.Author),
			120,
		),
	)
	currTest.testInfo.WriteHeader([]byte(output)) // nolint
	return
}

func appendResults(currTest testRun, results []testResult, showOnSuccess bool) {

	printLock.Lock()
	defer printLock.Unlock()

	failed := createHeader(currTest, results, showOnSuccess)

	for _, result := range results {
		output := ""

		if result.err == nil && !showOnSuccess {
			continue
		}

		data, err := ioutil.ReadAll(result.reader)
		if err != nil {
			panic(err)
		}

		output += goterm.Color(fmt.Sprintf("\nIteration [%d] log after %s", result.iteration+1, result.duration), goterm.MAGENTA) + "\n"
		if len(data) > 0 {
			output += fmt.Sprintf("  %s\n", strings.Replace(string(data), "\n", "\n  ", -1))
		} else {
			output += fmt.Sprintf("  <no log>\n")
		}

		if failed {
			output += fmt.Sprintf("%s\n", goterm.Color(fmt.Sprintf("  error: %s", result.err), goterm.RED))
		}

		if len(result.stack) > 0 {
			output += fmt.Sprintf("    Test panic:\n\n%s\n", string(result.stack))
		}
		currTest.testInfo.Write([]byte(output)) // nolint
	}
}

func averageTime(results []testResult) time.Duration {

	var total int
	for _, r := range results {
		total += int(r.duration)
	}

	return time.Duration(total / len(results)).Round(1 * time.Millisecond).Round(time.Millisecond)
}

func hasErrors(results []testResult) bool {

	for _, r := range results {

		if r.err != nil {
			return true
		}
	}

	return false
}
