package http

import (
	"fmt"
	"net/http"
	"time"
)

// CheckWithRetry performs HTTP check with retries
func CheckWithRetry(url string, expectedStatus int, retries int, delay time.Duration, debug bool) (int, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var lastErr error
	var lastStatus int

	for i := 0; i <= retries; i++ {
		if debug && i > 0 {
			fmt.Printf("[DEBUG] HTTP check retry %d/%d for %s\n", i, retries, url)
		}

		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			if i < retries {
				time.Sleep(delay)
				continue
			}
			return 0, fmt.Errorf("HTTP check failed after %d retries: %w", retries, err)
		}
		defer resp.Body.Close()

		lastStatus = resp.StatusCode

		if expectedStatus > 0 && resp.StatusCode != expectedStatus {
			if i < retries {
				time.Sleep(delay)
				continue
			}
			return resp.StatusCode, fmt.Errorf("expected status %d, got %d", expectedStatus, resp.StatusCode)
		}

		return resp.StatusCode, nil
	}

	return lastStatus, lastErr
}

