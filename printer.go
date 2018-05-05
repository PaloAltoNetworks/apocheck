package apocheck

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/mitchellh/go-wordwrap"

	"github.com/buger/goterm"
)

func printStatus(suite testSuite, status map[string]testRun) {

	ntests := len(suite)
	offset := 1

	var s string

	for i, t := range suite.sorted() {

		run, ok := status[t.Name]
		if !ok {
			s = "pending"
		} else if run.err == nil {
			s = goterm.Color("success", goterm.GREEN)
		} else {
			s = goterm.Color("failure", goterm.RED)
		}

		if _, err := goterm.Printf("[%s] %s", s, t.Name); err != nil {
			panic(err)
		}

		if ok {
			if _, err := goterm.Printf(" (%s)", run.elapsed.Round(1*time.Millisecond)); err != nil {
				panic(err)
			}
		}

		if i < ntests-1 {
			if _, err := goterm.Printf("\n"); err != nil {
				panic(err)
			}
		}
	}

	if len(status) < ntests {
		offset++
		if _, err := goterm.Printf("\n\n(%d/%d)", len(status), ntests); err != nil {
			panic(err)
		}
	}

	goterm.Flush()
	goterm.MoveCursorUp(len(suite) + offset)
}

func printResults(status map[string]testRun) {

	var failures int
	for n, run := range status {

		if run.err == nil {
			continue
		}

		failures++

		fmt.Println()
		fmt.Println(goterm.Bold(goterm.Color(fmt.Sprintf("[%s] %s: failed after %s", run.test.ID, n, run.elapsed.Round(time.Millisecond)), goterm.YELLOW)))
		fmt.Println()
		fmt.Println(wordwrap.WrapString(fmt.Sprintf("%s — %s", run.test.Description, run.test.Author), 80))
		data, err := ioutil.ReadAll(run.logger)
		if err != nil {
			panic(err)
		}

		if len(data) > 0 {
			fmt.Printf("\n  %s\n", strings.Replace(string(data), "\n", "\n  ", -1))
		} else {
			fmt.Println("<no logs>")
		}

		fmt.Println(goterm.Color(fmt.Sprintf("  error: %s", run.err), goterm.RED))
		fmt.Println()
	}

	if failures == 0 {
		fmt.Printf("\n%s\n", goterm.Color("All tests passed", goterm.GREEN))
	} else {
		var plural string
		if failures > 1 {
			plural = "s"
		}
		fmt.Printf("\n%s\n", goterm.Color(fmt.Sprintf("%d test%s failed", failures, plural), goterm.RED))
	}
}
