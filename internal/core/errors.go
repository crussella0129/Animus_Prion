package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrorCategory classifies errors for recovery strategy selection.
type ErrorCategory int

const (
	ErrCategoryUnknown ErrorCategory = iota
	ErrCategoryNetwork
	ErrCategoryAuth
	ErrCategoryRateLimit
	ErrCategoryModel
	ErrCategoryTool
	ErrCategoryPermission
	ErrCategoryParse
	ErrCategoryContextLength
)

func (c ErrorCategory) String() string {
	switch c {
	case ErrCategoryNetwork:
		return "NETWORK"
	case ErrCategoryAuth:
		return "AUTH"
	case ErrCategoryRateLimit:
		return "RATE_LIMIT"
	case ErrCategoryModel:
		return "MODEL"
	case ErrCategoryTool:
		return "TOOL"
	case ErrCategoryPermission:
		return "PERMISSION"
	case ErrCategoryParse:
		return "PARSE"
	case ErrCategoryContextLength:
		return "CONTEXT_LENGTH"
	default:
		return "UNKNOWN"
	}
}

// RecoveryStrategy indicates how to handle a classified error.
type RecoveryStrategy int

const (
	StrategyAbort RecoveryStrategy = iota
	StrategyRetry
	StrategyRetryBackoff
	StrategyReduceContext
	StrategySwitchProvider
	StrategyAskUser
)

func (s RecoveryStrategy) String() string {
	switch s {
	case StrategyRetry:
		return "RETRY"
	case StrategyRetryBackoff:
		return "RETRY_WITH_BACKOFF"
	case StrategyReduceContext:
		return "REDUCE_CONTEXT"
	case StrategySwitchProvider:
		return "SWITCH_PROVIDER"
	case StrategyAskUser:
		return "ASK_USER"
	default:
		return "ABORT"
	}
}

// ClassifiedError wraps an error with category and recovery information.
type ClassifiedError struct {
	Original error
	Category ErrorCategory
	Strategy RecoveryStrategy
	Retryable bool
}

func (e *ClassifiedError) Error() string {
	return fmt.Sprintf("[%s] %s (strategy: %s)", e.Category, e.Original, e.Strategy)
}

func (e *ClassifiedError) Unwrap() error {
	return e.Original
}

// errorPatterns maps regex patterns to error classifications.
var errorPatterns = []struct {
	pattern  *regexp.Regexp
	category ErrorCategory
	strategy RecoveryStrategy
	retry    bool
}{
	{regexp.MustCompile(`(?i)connection refused|timeout|ECONNREFUSED|ETIMEDOUT`), ErrCategoryNetwork, StrategyRetryBackoff, true},
	{regexp.MustCompile(`(?i)401|unauthorized|invalid.?api.?key|authentication`), ErrCategoryAuth, StrategyAskUser, false},
	{regexp.MustCompile(`(?i)429|rate.?limit|too.?many.?requests`), ErrCategoryRateLimit, StrategyRetryBackoff, true},
	{regexp.MustCompile(`(?i)model.?not.?found|invalid.?model`), ErrCategoryModel, StrategySwitchProvider, false},
	{regexp.MustCompile(`(?i)context.?length|token.?limit|maximum.?context|too.?long`), ErrCategoryContextLength, StrategyReduceContext, true},
	{regexp.MustCompile(`(?i)json|parse|unmarshal|decode|syntax`), ErrCategoryParse, StrategyRetry, true},
	{regexp.MustCompile(`(?i)permission|denied|forbidden|403`), ErrCategoryPermission, StrategyAskUser, false},
}

// ClassifyError categorizes an error and determines recovery strategy.
func ClassifyError(err error) *ClassifiedError {
	if err == nil {
		return nil
	}

	msg := err.Error()
	for _, ep := range errorPatterns {
		if ep.pattern.MatchString(msg) {
			return &ClassifiedError{
				Original:  err,
				Category:  ep.category,
				Strategy:  ep.strategy,
				Retryable: ep.retry,
			}
		}
	}

	return &ClassifiedError{
		Original:  err,
		Category:  ErrCategoryUnknown,
		Strategy:  StrategyAbort,
		Retryable: false,
	}
}

// IsRetryable checks whether a classified error can be retried.
// Uses errors.As to unwrap error chains (handles wrapped ClassifiedErrors).
func IsRetryable(err error) bool {
	var ce *ClassifiedError
	if errors.As(err, &ce) {
		return ce.Retryable
	}
	// Check the error message for common retryable patterns
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "rate limit")
}
