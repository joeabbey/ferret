// Package main demonstrates basic usage of the Ferret HTTP instrumentation library.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joeabbey/ferret/pkg/ferret"
)

func main() {
	// Create a new Ferret transport with default settings
	f := ferret.New()
	
	// Create an HTTP client using the Ferret transport
	client := &http.Client{
		Transport: f,
	}

	// Make a simple GET request
	fmt.Println("Making request to example.com...")
	req, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Get timing information
	result := ferret.GetResult(resp.Request)
	if result == nil {
		log.Fatal("No timing information available")
	}

	// Display timing breakdown
	fmt.Printf("\nTiming breakdown for %s:\n", req.URL)
	fmt.Printf("  DNS lookup:      %v\n", result.DNSDuration())
	fmt.Printf("  TCP connection:  %v\n", result.ConnectionDuration())
	fmt.Printf("  TLS handshake:   %v\n", result.TLSDuration())
	fmt.Printf("  Server process:  %v\n", result.ServerProcessingDuration())
	fmt.Printf("  Content transfer:%v\n", result.DataTransferDuration())
	fmt.Printf("  Total time:      %v\n", result.TotalDuration())
	fmt.Printf("\nHTTP Status: %d %s\n", resp.StatusCode, resp.Status)

	// Alternative: Use the String() method for a compact summary
	fmt.Printf("\nCompact summary: %s\n", result)

	// JSON output
	jsonData, err := result.MarshalJSON()
	if err == nil {
		fmt.Printf("\nJSON output: %s\n", string(jsonData))
	}
}

// Example output:
// Making request to example.com...
//
// Timing breakdown for https://example.com:
//   DNS lookup:      15.2ms
//   TCP connection:  45.3ms
//   TLS handshake:   89.1ms
//   Server process:  120.5ms
//   Content transfer:25.3ms
//   Total time:      295.4ms
//
// HTTP Status: 200 OK
//
// Compact summary: total=295.4ms dns=15.2ms connect=45.3ms tls=89.1ms ttfb=270.1ms
//
// JSON output: {"dns_ms":15.2,"connect_ms":45.3,"tls_ms":89.1,"ttfb_ms":270.1,"total_ms":295.4,"request_ms":145.8}