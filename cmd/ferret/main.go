package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/joeabbey/ferret/internal/aws"
	"github.com/joeabbey/ferret/pkg/ferret"
)

// Output formats
const (
	FormatText  = "text"
	FormatJSON  = "json"
	FormatShort = "short"
)

// Command modes
const (
	ModeSimple = "simple"
	ModeAWS    = "aws"
)

// Config holds the CLI configuration
type Config struct {
	Mode        string
	URL         string
	Iterations  int
	Concurrency int
	Format      string
	Timeout     time.Duration
	Method      string
	ShowDetails bool
}

// RequestResult holds the result of a single request
type RequestResult struct {
	Iteration    int           `json:"iteration"`
	Duration     time.Duration `json:"duration_ms"`
	Error        string        `json:"error,omitempty"`
	StatusCode   int           `json:"status_code,omitempty"`
	DNS          time.Duration `json:"dns_ms,omitempty"`
	Connect      time.Duration `json:"connect_ms,omitempty"`
	TLS          time.Duration `json:"tls_ms,omitempty"`
	TTFB         time.Duration `json:"ttfb_ms,omitempty"`
	DataTransfer time.Duration `json:"data_transfer_ms,omitempty"`
}

// Summary holds aggregate statistics
type Summary struct {
	URL        string          `json:"url"`
	Iterations int             `json:"iterations"`
	Successful int             `json:"successful"`
	Failed     int             `json:"failed"`
	Min        time.Duration   `json:"min_ms"`
	Max        time.Duration   `json:"max_ms"`
	Average    time.Duration   `json:"average_ms"`
	Median     time.Duration   `json:"median_ms"`
	P90        time.Duration   `json:"p90_ms"`
	P99        time.Duration   `json:"p99_ms"`
	Results    []RequestResult `json:"results,omitempty"`
}

// AWSResult holds results for AWS region testing
type AWSResult struct {
	Region  aws.Region `json:"region"`
	Summary Summary    `json:"summary"`
}

func main() {
	config := parseFlags()

	switch config.Mode {
	case ModeAWS:
		runAWSMode(config)
	default:
		runSimpleMode(config)
	}
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.Mode, "mode", ModeSimple, "Mode: simple or aws")
	flag.StringVar(&config.URL, "url", "", "URL to test (required for simple mode)")
	flag.IntVar(&config.Iterations, "iterations", 10, "Number of iterations")
	flag.IntVar(&config.Concurrency, "concurrency", 1, "Number of concurrent requests")
	flag.StringVar(&config.Format, "format", FormatText, "Output format: text, json, or short")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Request timeout")
	flag.StringVar(&config.Method, "method", "GET", "HTTP method")
	flag.BoolVar(&config.ShowDetails, "details", false, "Show detailed timing breakdown")

	flag.Parse()

	// Validate
	if config.Mode == ModeSimple && config.URL == "" {
		fmt.Fprintf(os.Stderr, "Error: -url is required for simple mode\n")
		flag.Usage()
		os.Exit(1)
	}

	if config.Concurrency < 1 {
		config.Concurrency = 1
	}

	if config.Iterations < 1 {
		config.Iterations = 1
	}

	return config
}

func runSimpleMode(config Config) {
	// Create Ferret transport
	transport := ferret.New(
		ferret.WithTimeout(10*time.Second, config.Timeout),
	)
	client := &http.Client{Transport: transport}

	results := make([]RequestResult, 0, config.Iterations)
	resultsChan := make(chan RequestResult, config.Iterations)

	// Use semaphore for concurrency control
	sem := make(chan struct{}, config.Concurrency)
	var wg sync.WaitGroup

	// Run requests
	for i := 0; i < config.Iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := performRequest(client, config.URL, config.Method, iteration)
			resultsChan <- result
		}(i)
	}

	// Wait and collect results
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for result := range resultsChan {
		results = append(results, result)
		if config.Format == FormatText && !config.ShowDetails {
			printProgress(result)
		}
	}

	// Sort results by iteration
	sort.Slice(results, func(i, j int) bool {
		return results[i].Iteration < results[j].Iteration
	})

	// Generate summary
	summary := generateSummary(config.URL, results)

	// Output results
	switch config.Format {
	case FormatJSON:
		printJSON(summary)
	case FormatShort:
		printShort(summary)
	default:
		printText(summary, config.ShowDetails)
	}
}

func runAWSMode(config Config) {
	regions := aws.GetRegions()
	transport := ferret.New(
		ferret.WithTimeout(5*time.Second, 10*time.Second),
	)
	client := &http.Client{Transport: transport}

	var mu sync.Mutex
	var wg sync.WaitGroup
	awsResults := make([]AWSResult, 0, len(regions))

	// Test each region
	for _, region := range regions {
		wg.Add(1)
		go func(r aws.Region) {
			defer wg.Done()

			if config.Format == FormatText {
				fmt.Printf("Testing %s (%s)...\n", r.ID, r.Name)
			}

			results := make([]RequestResult, 0, config.Iterations)
			for i := 0; i < config.Iterations; i++ {
				result := performRequest(client, r.Endpoint, "GET", i)
				results = append(results, result)
			}

			summary := generateSummary(r.Endpoint, results)

			mu.Lock()
			awsResults = append(awsResults, AWSResult{
				Region:  r,
				Summary: summary,
			})
			mu.Unlock()
		}(region)
	}

	wg.Wait()

	// Sort by average latency
	sort.Slice(awsResults, func(i, j int) bool {
		return awsResults[i].Summary.Average < awsResults[j].Summary.Average
	})

	// Output results
	switch config.Format {
	case FormatJSON:
		printJSON(awsResults)
	default:
		printAWSText(awsResults)
	}
}

func performRequest(client *http.Client, url, method string, iteration int) RequestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return RequestResult{
			Iteration: iteration,
			Error:     err.Error(),
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return RequestResult{
			Iteration: iteration,
			Duration:  duration,
			Error:     err.Error(),
		}
	}
	defer resp.Body.Close()

	// Consume body
	_, _ = io.Copy(io.Discard, resp.Body)

	// Get detailed timing
	result := RequestResult{
		Iteration:  iteration,
		Duration:   duration,
		StatusCode: resp.StatusCode,
	}

	if ferretResult := ferret.GetResult(resp.Request); ferretResult != nil {
		result.DNS = ferretResult.DNSDuration()
		result.Connect = ferretResult.ConnectionDuration()
		result.TLS = ferretResult.TLSDuration()
		result.TTFB = ferretResult.TTFB()
		result.DataTransfer = ferretResult.DataTransferDuration()
	}

	return result
}

func generateSummary(url string, results []RequestResult) Summary {
	summary := Summary{
		URL:        url,
		Iterations: len(results),
		Results:    results,
	}

	// Calculate statistics
	var successful []time.Duration
	for _, r := range results {
		if r.Error == "" {
			summary.Successful++
			successful = append(successful, r.Duration)
		} else {
			summary.Failed++
		}
	}

	if len(successful) > 0 {
		sort.Slice(successful, func(i, j int) bool {
			return successful[i] < successful[j]
		})

		summary.Min = successful[0]
		summary.Max = successful[len(successful)-1]

		// Average
		var sum time.Duration
		for _, d := range successful {
			sum += d
		}
		summary.Average = sum / time.Duration(len(successful))

		// Median
		if len(successful)%2 == 0 {
			summary.Median = (successful[len(successful)/2-1] + successful[len(successful)/2]) / 2
		} else {
			summary.Median = successful[len(successful)/2]
		}

		// Percentiles
		p90Index := int(float64(len(successful)) * 0.9)
		if p90Index >= len(successful) {
			p90Index = len(successful) - 1
		}
		summary.P90 = successful[p90Index]

		p99Index := int(float64(len(successful)) * 0.99)
		if p99Index >= len(successful) {
			p99Index = len(successful) - 1
		}
		summary.P99 = successful[p99Index]
	}

	return summary
}

func printProgress(result RequestResult) {
	if result.Error != "" {
		fmt.Printf("  #%d: ERROR: %s\n", result.Iteration+1, result.Error)
	} else {
		fmt.Printf("  #%d: %v (status: %d)\n", result.Iteration+1, result.Duration.Round(time.Millisecond), result.StatusCode)
	}
}

func printText(summary Summary, showDetails bool) {
	fmt.Printf("\n=== Summary for %s ===\n", summary.URL)
	fmt.Printf("Iterations: %d (Success: %d, Failed: %d)\n", summary.Iterations, summary.Successful, summary.Failed)

	if summary.Successful > 0 {
		fmt.Printf("\nLatency Statistics:\n")
		fmt.Printf("  Min:     %v\n", summary.Min.Round(time.Millisecond))
		fmt.Printf("  Max:     %v\n", summary.Max.Round(time.Millisecond))
		fmt.Printf("  Average: %v\n", summary.Average.Round(time.Millisecond))
		fmt.Printf("  Median:  %v\n", summary.Median.Round(time.Millisecond))
		fmt.Printf("  P90:     %v\n", summary.P90.Round(time.Millisecond))
		fmt.Printf("  P99:     %v\n", summary.P99.Round(time.Millisecond))
	}

	if showDetails && summary.Successful > 0 {
		fmt.Printf("\nDetailed Results:\n")
		for _, r := range summary.Results {
			if r.Error == "" {
				fmt.Printf("  #%d: Total: %v", r.Iteration+1, r.Duration.Round(time.Millisecond))
				if r.DNS > 0 {
					fmt.Printf(" (DNS: %v, Connect: %v, TLS: %v, TTFB: %v, Transfer: %v)",
						r.DNS.Round(time.Millisecond),
						r.Connect.Round(time.Millisecond),
						r.TLS.Round(time.Millisecond),
						r.TTFB.Round(time.Millisecond),
						r.DataTransfer.Round(time.Millisecond))
				}
				fmt.Printf(" [%d]\n", r.StatusCode)
			}
		}
	}
}

func printShort(summary Summary) {
	if summary.Successful > 0 {
		fmt.Printf("%s: avg=%v min=%v max=%v p90=%v p99=%v (success=%d/%d)\n",
			summary.URL,
			summary.Average.Round(time.Millisecond),
			summary.Min.Round(time.Millisecond),
			summary.Max.Round(time.Millisecond),
			summary.P90.Round(time.Millisecond),
			summary.P99.Round(time.Millisecond),
			summary.Successful,
			summary.Iterations)
	} else {
		fmt.Printf("%s: all requests failed (%d/%d)\n", summary.URL, summary.Failed, summary.Iterations)
	}
}

func printAWSText(results []AWSResult) {
	fmt.Println("\n=== AWS Region Latency Test Results ===")
	fmt.Printf("%-20s %-30s %10s %10s %10s\n", "Region", "Name", "Average", "Min", "Max")
	fmt.Println(stringRepeat("-", 80))

	for _, r := range results {
		if r.Summary.Successful > 0 {
			fmt.Printf("%-20s %-30s %10v %10v %10v\n",
				r.Region.ID,
				r.Region.Name,
				r.Summary.Average.Round(time.Millisecond),
				r.Summary.Min.Round(time.Millisecond),
				r.Summary.Max.Round(time.Millisecond))
		} else {
			fmt.Printf("%-20s %-30s %10s %10s %10s\n",
				r.Region.ID,
				r.Region.Name,
				"FAILED", "-", "-")
		}
	}

	if len(results) > 0 && results[0].Summary.Successful > 0 {
		fmt.Printf("\nFastest region: %s (%s) with average latency of %v\n",
			results[0].Region.ID,
			results[0].Region.Name,
			results[0].Summary.Average.Round(time.Millisecond))
	}
}

func printJSON(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func stringRepeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

