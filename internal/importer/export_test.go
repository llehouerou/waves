package importer

import "context"

// RetryWithBackoff exposes retryWithBackoff for testing.
var RetryWithBackoff = retryWithBackoff

// IsRetryableError exposes isRetryableError for testing.
var IsRetryableError = isRetryableError

// Test constants exposed for verification.
const (
	TestMaxRetries       = maxRetries
	TestInitialBackoff   = initialBackoff
	TestMaxBackoff       = maxBackoff
	TestOperationTimeout = operationTimeout
)

// RetryWithBackoffFunc is the function signature for retryWithBackoff.
type RetryWithBackoffFunc = func(ctx context.Context, operation string, fn func() error) error
