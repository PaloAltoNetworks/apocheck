package apocheck

import (
	"fmt"
	"io/ioutil"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"go.aporeto.io/underwater/ibatcher"

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
				fmt.Sprintf("%s FAIL %s",
					curTest.test.id,
					curTest.test.Name,
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

	sname := "none"
	if currTest.test.SuiteName != "" {
		sname = currTest.test.SuiteName
	}

	if !failed && !showOnSuccess {
		output := goterm.Color(
			fmt.Sprintf("%s (%s): %s %s",
				resultString,
				currTest.test.id,
				currTest.test.Name,
				goterm.Color(fmt.Sprintf("it: %d, avg: %s, suite: %s", len(results), averageTime(results), sname), goterm.BLUE),
			),
			goterm.GREEN,
		)
		currTest.testInfo.WriteHeader([]byte(output)) // nolint
		return
	}

	color := goterm.GREEN
	if failed {
		color = goterm.YELLOW
	}

	output := fmt.Sprintf("%s\n%s",
		goterm.Bold(
			goterm.Color(
				fmt.Sprintf("ID: %s : %s : %s",
					currTest.test.id,
					resultString,
					currTest.test.Name,
				),
				color),
		),
		wordwrap.WrapString(fmt.Sprintf("%s — %s\n", currTest.test.Description, currTest.test.Author),
			120,
		),
	)
	currTest.testInfo.WriteHeader([]byte(output)) // nolint
	return failed
}

func appendResults(run testRun, results []testResult, showOnSuccess bool, batcher ibatcher.Batcher) {

	if batcher != nil {
		batcher.Push(
			statsReport{
				ID:       run.test.id,
				Suite:    run.test.SuiteName,
				Name:     run.test.Name,
				Duration: int(averageTime(results)),
				Value: func() int {
					if hasErrors(results) {
						return 0
					}
					return 1
				}(),
			}.point(run.name),
		)
	}

	printLock.Lock()
	defer printLock.Unlock()

	failed := createHeader(run, results, showOnSuccess)

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
		run.testInfo.Write([]byte(output)) // nolint
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
