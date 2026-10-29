package client

import "fmt"

// HTTPStatusError preserves the remote status so callers can distinguish a
// gateway/request timeout from authentication and other HTTP failures.
type HTTPStatusError struct{ StatusCode int }

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("API request failed with status: %d", e.StatusCode)
}
