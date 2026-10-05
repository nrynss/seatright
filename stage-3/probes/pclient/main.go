// Command probe is a tiny stdlib HTTP client bundled into the image at
// /app/probe. It runs inside `docker exec` so the service container itself
// can be probed while its external network is off (`--network none`):
// localhost inside the container's own namespace still reaches the service,
// while no outbound traffic can leave.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	method := flag.String("method", "GET", "HTTP method")
	url := flag.String("url", "", "URL to request")
	body := flag.String("body", "", "request body (POST/PATCH)")
	token := flag.String("token", "", "bearer token")
	key := flag.String("key", "", "Idempotency-Key header")
	timeout := flag.Duration("timeout", 10*time.Second, "request timeout")
	flag.Parse()
	if *url == "" {
		fmt.Fprintln(os.Stderr, "missing -url")
		os.Exit(2)
	}
	var reader io.Reader
	if *body != "" {
		reader = strings.NewReader(*body)
	}
	req, err := http.NewRequest(*method, *url, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request:", err)
		os.Exit(2)
	}
	if *body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if *token != "" {
		req.Header.Set("Authorization", "Bearer "+*token)
	}
	if *key != "" {
		req.Header.Set("Idempotency-Key", *key)
	}
	client := &http.Client{Timeout: *timeout}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "do:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	fmt.Printf("HTTP %d\n", resp.StatusCode)
	for _, h := range []string{"Content-Type"} {
		fmt.Printf("%s: %s\n", h, resp.Header.Get(h))
	}
	fmt.Println(string(raw))
}
