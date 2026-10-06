package awsx

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/smithy-go"
)

func apiError(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: "from a test"}
}

func TestErrorCode(t *testing.T) {
	if got := ErrorCode(apiError("NoSuchEntity")); got != "NoSuchEntity" {
		t.Errorf("ErrorCode = %q, want NoSuchEntity", got)
	}
	// SDK errors arrive wrapped several layers deep.
	wrapped := fmt.Errorf("operation error IAM: GetRole: %w", apiError("NoSuchEntity"))
	if got := ErrorCode(wrapped); got != "NoSuchEntity" {
		t.Errorf("ErrorCode of a wrapped error = %q, want NoSuchEntity", got)
	}
	if got := ErrorCode(errors.New("plain")); got != "" {
		t.Errorf("ErrorCode of a non-AWS error = %q, want empty", got)
	}
	if got := ErrorCode(nil); got != "" {
		t.Errorf("ErrorCode(nil) = %q, want empty", got)
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		code                                        string
		notFound, alreadyExists, denied, catchingUp bool
	}{
		{code: "ResourceNotFoundException", notFound: true, catchingUp: true},
		{code: "NoSuchBucket", notFound: true, catchingUp: true},
		{code: "NotFound", notFound: true, catchingUp: true},
		{code: "NoSuchEntity", notFound: true, catchingUp: true},
		{code: "NotFoundException", notFound: true, catchingUp: true},
		{code: "ResourceInUseException", alreadyExists: true},
		{code: "BucketAlreadyOwnedByYou", alreadyExists: true},
		{code: "EntityAlreadyExists", alreadyExists: true},
		{code: "DuplicateRecordException", alreadyExists: true},
		{code: "AccessDenied", denied: true, catchingUp: true},
		{code: "AccessDeniedException", denied: true, catchingUp: true},
		// Someone else owns the bucket name. That is a naming clash, not "ours already exists".
		{code: "BucketAlreadyExists"},
		{code: "ThrottlingException"},
		{code: "ValidationException"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := fmt.Errorf("wrapped: %w", apiError(tt.code))
			if got := IsNotFound(err); got != tt.notFound {
				t.Errorf("IsNotFound = %v, want %v", got, tt.notFound)
			}
			if got := IsAlreadyExists(err); got != tt.alreadyExists {
				t.Errorf("IsAlreadyExists = %v, want %v", got, tt.alreadyExists)
			}
			if got := IsAccessDenied(err); got != tt.denied {
				t.Errorf("IsAccessDenied = %v, want %v", got, tt.denied)
			}
			if got := IsNotYetConsistent(err); got != tt.catchingUp {
				t.Errorf("IsNotYetConsistent = %v, want %v", got, tt.catchingUp)
			}
		})
	}
}

func TestErrorClassificationOfNonAWSErrors(t *testing.T) {
	for _, err := range []error{nil, errors.New("connection reset")} {
		if IsNotFound(err) || IsAlreadyExists(err) || IsAccessDenied(err) || IsNotYetConsistent(err) {
			t.Errorf("%v was classified as an AWS error", err)
		}
	}
}
