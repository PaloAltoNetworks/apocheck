package apocheck

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/mitchellh/go-wordwrap"

	"github.com/buger/goterm"
)

func printStatus(suite testSuite, status map[string]testRun, completed int, stress int) {

	ntests := len(suite) * stress
	offset := 1

	var s string

	for i, t := range suite.sorted() {

		run, ok := status[t.Name]
		if !ok || len(run.errs) < stress {
			s = "pending"
		} else if hasErrors(run.errs) {
			s = goterm.Color("failure", goterm.RED)
		} else {
			s = goterm.Color("success", goterm.GREEN)
		}

		if _, err := goterm.Printf("[%s] (%d/%d) %s", s, len(run.errs), stress, t.Name); err != nil {
			panic(err)
		}

		if ok && len(run.errs) == stress {
			if _, err := goterm.Print(goterm.Color(fmt.Sprintf(" - avg: %s", averageTime(run.durations)), goterm.BLUE)); err != nil {
				panic(err)
			}
		}

		if i < len(suite)-1 {
			if _, err := goterm.Printf("\n"); err != nil {
				panic(err)
			}
		}
	}

	if completed < ntests {
		offset++
		if _, err := goterm.Printf("\n\n(%d/%d)", completed, ntests); err != nil {
			panic(err)
		}
	}

	goterm.Flush()
	goterm.MoveCursorUp(len(suite) + offset)
}

func printResults(status map[string]testRun, showOnSuccess bool) {

	var failures int
	for n, run := range status {

		failed := hasErrors(run.errs)
		if !failed && !showOnSuccess {
			continue
		}

		color := goterm.GREEN
		if failed {
			failures++
			color = goterm.YELLOW
		}

		fmt.Println()
		fmt.Println(goterm.Bold(goterm.Color(fmt.Sprintf("[%s] %s", run.test.ID, n), color)))
		fmt.Println()
		fmt.Println(wordwrap.WrapString(fmt.Sprintf("%s — %s", run.test.Description, run.test.Author), 80))
		fmt.Println()

		for i, log := range run.loggers {

			if run.errs[i] == nil && !showOnSuccess {
				continue
			}

			data, err := ioutil.ReadAll(log)
			if err != nil {
				panic(err)
			}

			fmt.Printf("iteration %d log after %s\n", i, run.durations[i].Round(time.Millisecond))
			if len(data) > 0 {
				fmt.Printf("\n  %s\n", strings.Replace(string(data), "\n", "\n  ", -1))
			} else {
				fmt.Println("\n  <no logs>")
			}

			if failed {
				fmt.Println(goterm.Color(fmt.Sprintf("  error: %s", run.errs[i]), goterm.RED))
			}
			fmt.Println()
		}
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

func averageTime(durations []time.Duration) time.Duration {

	var total int
	for _, d := range durations {
		total += int(d)
	}

	return time.Duration(total / len(durations)).Round(1 * time.Millisecond)
}

func hasErrors(errs []error) bool {

	for _, e := range errs {

		if e != nil {
			return true
		}
	}

	return false
}
