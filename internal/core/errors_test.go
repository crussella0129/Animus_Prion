package core

import (
	"errors"
	"testing"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err      string
		category ErrorCategory
		retry    bool
	}{
		{"connection refused", ErrCategoryNetwork, true},
		{"401 unauthorized", ErrCategoryAuth, false},
		{"429 rate limit exceeded", ErrCategoryRateLimit, true},
		{"model not found", ErrCategoryModel, false},
		{"context length exceeded", ErrCategoryContextLength, true},
		{"json parse error", ErrCategoryParse, true},
		{"permission denied", ErrCategoryPermission, false},
		{"something unknown", ErrCategoryUnknown, false},
	}

	for _, tt := range tests {
		ce := ClassifyError(errors.New(tt.err))
		if ce.Category != tt.category {
			t.Errorf("ClassifyError(%q) category = %v, want %v", tt.err, ce.Category, tt.category)
		}
		if ce.Retryable != tt.retry {
			t.Errorf("ClassifyError(%q) retryable = %v, want %v", tt.err, ce.Retryable, tt.retry)
		}
	}
}

func TestClassifyNilError(t *testing.T) {
	if ClassifyError(nil) != nil {
		t.Error("ClassifyError(nil) should return nil")
	}
}
