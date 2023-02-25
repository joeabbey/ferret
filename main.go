package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/joeabbey/ferret/pkg/ferret"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

func main() {
	prefix := "https://ec2."
	suffix := ".amazonaws.com/ping"
	iterations := 10
	endpoints := []string{
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-northeast-3",
		"ap-south-1",
		"ap-southeast-1",
		"ap-southeast-2",
		"ca-central-1",
		"eu-central-1",
		"eu-north-1",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"sa-east-1",
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
	}
	ep := startUI(prefix, endpoints, suffix, iterations)
	fmt.Printf("%s\n", ep)

}

func startUI(prefix string, endpoints []string, suffix string, iterations int) string {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	ep := findNearestEndpoint(prefix, endpoints, suffix, iterations)

	uiEvents := ui.PollEvents()

	done := false
	for !done {
		e := <-uiEvents
		switch e.ID {
		case "r":
			ep = findNearestEndpoint(prefix, endpoints, suffix, iterations)
		case "q", "<C-c>":
			done = true
		}
	}
	return ep
}

func findNearestEndpoint(prefix string, endpoints []string, suffix string, iterations int) string {
	const maxConcurrent = 64
	sem := make(chan bool, maxConcurrent)
	var mtx sync.Mutex
	var wg sync.WaitGroup
	var results [][]time.Duration

	tableView := widgets.NewTable()
	tableView.ColumnWidths = []int{15, 7}
	tableView.Rows = append(tableView.Rows, []string{
		"Endpoint",
		"avg",
	})

	for i := 0; i < iterations; i++ {
		tableView.Rows[0] = append(tableView.Rows[0], strconv.Itoa(i+1))
	}

	for e, endpoint := range endpoints {
		tableView.Rows = append(tableView.Rows, make([]string, iterations+2))
		tableView.Rows[e+1][0] = endpoint
		results = append(results, make([]time.Duration, iterations))
		tableView.ColumnWidths = append(tableView.ColumnWidths, 7)
	}

	tableView.SetRect(2, 2, (iterations*8)+27, len(endpoints)*2+1)
	tableView.TextStyle = ui.NewStyle(ui.ColorWhite)
	tableView.TextAlignment = ui.AlignCenter
	ui.Render(tableView)

	for e, endpoint := range endpoints {
		for iter := 0; iter < iterations; iter++ {
			sem <- true
			go func(iter int, e int, endpoint string) {
				wg.Add(1)
				defer wg.Done()
				defer func() { <-sem }()

				d, err := measureDuration(prefix + endpoint + suffix)
				mtx.Lock()
				results[e][iter] = d
				if err != nil {
					tableView.Rows[e+1][iter+2] = "???"
				} else {
					tableView.Rows[e+1][iter+2] = d.Truncate(time.Millisecond).String()
				}
				ui.Render(tableView)
				mtx.Unlock()

			}(iter, e, endpoint)
		}
	}

	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	wg.Wait()

	computeAverages(results, tableView)
	sortByAverage(tableView)
	colorizeRows(tableView)
	ui.Render(tableView)

	ep := tableView.Rows[1][0]
	return ep
}

func measureDuration(url string) (time.Duration, error) {
	f := ferret.NewFerret()
	client := &http.Client{Transport: f}

	resp, err := client.Get(url)
	if err != nil {
		return time.Duration(0), err
	}
	defer resp.Body.Close()

	output := ioutil.Discard
	io.Copy(output, resp.Body)

	return f.ConnDuration(), nil
}

func computeAverages(results [][]time.Duration, tableView *widgets.Table) {
	for i, row := range results {
		var sum time.Duration
		for _, result := range row {
			if result != time.Duration(0) {
				sum = sum + result
			}
		}

		//No this isn't a duration, but it makes the types match
		iterations := time.Duration(len(row) - 1)
		average := sum / iterations

		tableView.Rows[i+1][1] = average.Truncate(time.Millisecond).String()
	}
}

func sortByAverage(tableView *widgets.Table) {
	sort.Slice(tableView.Rows, func(i, j int) bool {
		ti, ei := time.ParseDuration(tableView.Rows[i][1])
		if ei != nil {
			return true
		}

		tj, ej := time.ParseDuration(tableView.Rows[j][1])
		if ej != nil {
			return false
		}

		return ti < tj
	})
}

func colorizeRows(tableView *widgets.Table) {

	d100ms, _ := time.ParseDuration("100ms")
	d250ms, _ := time.ParseDuration("250ms")

	for i := range tableView.Rows {
		avg, err := time.ParseDuration(tableView.Rows[i][1])
		if err != nil {
			continue
		}
		if avg < d100ms {
			tableView.RowStyles[i] = ui.NewStyle(ui.ColorGreen)
		} else if avg < d250ms {
			tableView.RowStyles[i] = ui.NewStyle(ui.ColorYellow)
		} else {
			tableView.RowStyles[i] = ui.NewStyle(ui.ColorRed)
		}
	}
}
