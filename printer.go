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

func printResults(currTest testRun, results []testResult, showOnSuccess bool) {

	printLock.Lock()
	defer printLock.Unlock()

	var failures int

	failed := hasErrors(results)

	resultString := "FAIL"
	if !failed {
		resultString = "PASS"
	}

	if !failed && !showOnSuccess {
		fmt.Printf("%s\n",
			goterm.Color(
				fmt.Sprintf("%s %s %s (variant %s) %s",
					currTest.test.id,
					resultString,
					currTest.test.Name,
					currTest.testInfo.testVariant,
					goterm.Color(fmt.Sprintf("it: %d, avg: %s", len(results), averageTime(results)), goterm.BLUE),
				),
				goterm.GREEN,
			),
		)
		return
	}

	color := goterm.GREEN
	if failed {
		failures++
		color = goterm.YELLOW
	}

	fmt.Println()
	fmt.Println(goterm.Bold(goterm.Color(fmt.Sprintf("%s %s %s (variant %s)", currTest.test.id, resultString, currTest.test.Name, currTest.testInfo.testVariant), color)))
	fmt.Println()
	fmt.Println(wordwrap.WrapString(fmt.Sprintf("%s — %s", currTest.test.Description, currTest.test.Author), 80))
	fmt.Println()

	for _, result := range results {

		if result.err == nil && !showOnSuccess {
			continue
		}

		data, err := ioutil.ReadAll(result.reader)
		if err != nil {
			panic(err)
		}

		fmt.Println(goterm.Color(fmt.Sprintf("iteration %d log after %s\n", result.iteration+1, result.duration), goterm.MAGENTA))
		if len(data) > 0 {
			fmt.Printf("\n  %s\n", strings.Replace(string(data), "\n", "\n  ", -1))
		} else {
			fmt.Println()
			fmt.Println("  <no log>")
			fmt.Println()
		}

		if failed {
			fmt.Println(goterm.Color(fmt.Sprintf("  error: %s", result.err), goterm.RED))
		}

		if len(result.stack) > 0 {
			fmt.Println()
			fmt.Println("  Test panic:")
			fmt.Println()
			fmt.Println(string(result.stack))
		}
		fmt.Println()
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
