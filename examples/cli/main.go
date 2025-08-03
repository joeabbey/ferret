// Package main demonstrates a CLI tool using Ferret for HTTP performance testing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/joeabbey/ferret/pkg/ferret"
)

type Result struct {
	URL      string                 `json:"url"`
	Status   int                    `json:"status"`
	Error    string                 `json:"error,omitempty"`
	Timings  map[string]float64     `json:"timings_ms"`
	Headers  map[string]string      `json:"headers,omitempty"`
}

func main() {
	var (
		iterations  = flag.Int("n", 3, "Number of requests per URL")
		concurrent  = flag.Int("c", 1, "Number of concurrent requests")
		timeout     = flag.Duration("timeout", 10*time.Second, "Request timeout")
		jsonOutput  = flag.Bool("json", false, "Output results as JSON")
		showHeaders = flag.Bool("headers", false, "Show response headers")
		method      = flag.String("method", "GET", "HTTP method")
		follow      = flag.Bool("L", false, "Follow redirects")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <url> [url...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nA simple HTTP performance testing tool powered by Ferret.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s https://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -n 10 -c 3 https://example.com https://golang.org\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -json -headers https://api.example.com/health\n", os.Args[0])
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	urls := flag.Args()

	// Create Ferret transport
	f := ferret.New(
		ferret.WithTimeout(*timeout/2, *timeout),
		ferret.WithKeepAlives(true),
	)

	// Create HTTP client
	client := &http.Client{
		Transport: f,
		Timeout:   *timeout,
	}

	if !*follow {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Perform tests
	results := make([]Result, 0, len(urls)**iterations)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore for concurrency control
	sem := make(chan struct{}, *concurrent)

	for _, url := range urls {
		for i := 0; i < *iterations; i++ {
			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore

			go func(url string, iter int) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore

				result := testURL(client, *method, url, *showHeaders)
				
				mu.Lock()
				results = append(results, result)
				mu.Unlock()

				if !*jsonOutput {
					printResult(result, iter+1)
				}
			}(url, i)
		}
	}

	wg.Wait()

	// Output results
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(results)
	} else {
		printSummary(results, *iterations)
	}
}

func testURL(client *http.Client, method, url string, includeHeaders bool) Result {
	result := Result{
		URL:     url,
		Timings: make(map[string]float64),
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Add custom user agent
	req.Header.Set("User-Agent", "Ferret-CLI/1.0")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.Status = resp.StatusCode

	// Get timing information
	if timing := ferret.GetResult(resp.Request); timing != nil {
		result.Timings["dns_ms"] = float64(timing.DNSDuration()) / float64(time.Millisecond)
		result.Timings["connect_ms"] = float64(timing.ConnectionDuration()) / float64(time.Millisecond)
		result.Timings["tls_ms"] = float64(timing.TLSDuration()) / float64(time.Millisecond)
		result.Timings["ttfb_ms"] = float64(timing.TTFB()) / float64(time.Millisecond)
		result.Timings["total_ms"] = float64(timing.TotalDuration()) / float64(time.Millisecond)
		result.Timings["server_ms"] = float64(timing.ServerProcessingDuration()) / float64(time.Millisecond)
		result.Timings["transfer_ms"] = float64(timing.DataTransferDuration()) / float64(time.Millisecond)
	}

	// Include headers if requested
	if includeHeaders {
		result.Headers = make(map[string]string)
		for k, v := range resp.Header {
			if len(v) > 0 {
				result.Headers[k] = v[0]
			}
		}
	}

	return result
}

func printResult(r Result, iteration int) {
	if r.Error != "" {
		fmt.Printf("[%d] %s - ERROR: %s\n", iteration, r.URL, r.Error)
		return
	}

	fmt.Printf("[%d] %s - Status: %d, Total: %.1fms (DNS: %.1fms, Connect: %.1fms, TLS: %.1fms, TTFB: %.1fms)\n",
		iteration, r.URL, r.Status,
		r.Timings["total_ms"],
		r.Timings["dns_ms"],
		r.Timings["connect_ms"],
		r.Timings["tls_ms"],
		r.Timings["ttfb_ms"],
	)
}

func printSummary(results []Result, _ int) {
	fmt.Println("\n=== Summary ===")

	// Group by URL
	byURL := make(map[string][]Result)
	for _, r := range results {
		byURL[r.URL] = append(byURL[r.URL], r)
	}

	// Sort URLs for consistent output
	urls := make([]string, 0, len(byURL))
	for url := range byURL {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	for _, url := range urls {
		urlResults := byURL[url]
		fmt.Printf("\n%s:\n", url)

		// Calculate statistics
		var totalTime, minTime, maxTime float64
		var successCount int
		minTime = 999999

		for _, r := range urlResults {
			if r.Error == "" {
				successCount++
				t := r.Timings["total_ms"]
				totalTime += t
				if t < minTime {
					minTime = t
				}
				if t > maxTime {
					maxTime = t
				}
			}
		}

		fmt.Printf("  Requests: %d successful, %d failed\n", successCount, len(urlResults)-successCount)
		
		if successCount > 0 {
			avgTime := totalTime / float64(successCount)
			fmt.Printf("  Response times: min=%.1fms, avg=%.1fms, max=%.1fms\n", minTime, avgTime, maxTime)
			
			// Show average phase times
			var avgDNS, avgConnect, avgTLS, avgTTFB float64
			for _, r := range urlResults {
				if r.Error == "" {
					avgDNS += r.Timings["dns_ms"]
					avgConnect += r.Timings["connect_ms"]
					avgTLS += r.Timings["tls_ms"]
					avgTTFB += r.Timings["ttfb_ms"]
				}
			}
			
			fmt.Printf("  Average phases: DNS=%.1fms, Connect=%.1fms, TLS=%.1fms, TTFB=%.1fms\n",
				avgDNS/float64(successCount),
				avgConnect/float64(successCount),
				avgTLS/float64(successCount),
				avgTTFB/float64(successCount),
			)
		}
	}
}