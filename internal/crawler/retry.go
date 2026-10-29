package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"treehole/internal/client"
)

const (
	maxTimeoutRetries = 2
	maxRetryTimeout   = 10 * time.Minute
)

// retryTimeout retries only the failed request, keeping its page and limit
// unchanged. Each attempt owns its timeout; the shared client's policy stays put.
func retryTimeout[T any](c *client.Client, label string, fetch func(*client.Client) (T, error)) (T, error) {
	timeout := c.GetHttpClient().Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	for attempt := 0; ; attempt++ {
		value, err := fetch(c.WithTimeout(timeout))
		if err == nil {
			return value, nil
		}
		var networkError net.Error
		timedOut := errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout())
		if !timedOut {
			return value, err
		}
		if attempt == maxTimeoutRetries {
			return value, fmt.Errorf("%s: timeout after %d attempts: %w", label, attempt+1, err)
		}
		if timeout < maxRetryTimeout {
			if timeout >= maxRetryTimeout/2 {
				timeout = maxRetryTimeout
			} else {
				timeout *= 2
			}
		}
		log.Printf("[Crawler] %s 超时，重试 %d/%d，超时上限 %s", label, attempt+1, maxTimeoutRetries, timeout)
	}
}
