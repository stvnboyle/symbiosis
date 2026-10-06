package awsx

import (
	"errors"
	"slices"

	"github.com/aws/smithy-go"
)

// AWS has no shared error vocabulary: each service names the same situation
// differently. These lists cover the services symbiosis uses and grow with each module.
var (
	notFoundCodes = []string{
		"ResourceNotFoundException", // DynamoDB
		"NoSuchBucket",              // S3
		"NotFound",                  // S3 HeadBucket, which has no response body to carry a fuller code
		"NoSuchEntity",              // IAM
		"NotFoundException",         // Budgets
	}
	alreadyExistsCodes = []string{
		"ResourceInUseException",   // DynamoDB: table exists
		"BucketAlreadyOwnedByYou",  // S3: exists and is ours. BucketAlreadyExists means someone else has the name
		"EntityAlreadyExists",      // IAM
		"DuplicateRecordException", // Budgets
	}
	accessDeniedCodes = []string{
		"AccessDenied",          // IAM, STS, S3
		"AccessDeniedException", // DynamoDB, Budgets
	}
)

// ErrorCode returns the AWS error code inside err, or "" if err is not an AWS API
// error.
func ErrorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	return ""
}

// IsNotFound reports whether err means the resource does not exist.
func IsNotFound(err error) bool {
	return hasCode(err, notFoundCodes)
}

// IsAlreadyExists reports whether err means the resource already exists and belongs
// to this account.
func IsAlreadyExists(err error) bool {
	return hasCode(err, alreadyExistsCodes)
}

// IsAccessDenied reports whether err means the caller lacks permission.
func IsAccessDenied(err error) bool {
	return hasCode(err, accessDeniedCodes)
}

// IsNotYetConsistent reports whether err is what AWS returns while a change is still
// spreading through a service: the thing just created cannot be found yet, or the
// permission just granted is not honoured yet.
//
// AWS has no error code for "try again shortly", so this is only meaningful
// immediately after a write. The same error at any other time is a real failure.
func IsNotYetConsistent(err error) bool {
	return IsNotFound(err) || IsAccessDenied(err)
}

func hasCode(err error, codes []string) bool {
	code := ErrorCode(err)
	return code != "" && slices.Contains(codes, code)
}
