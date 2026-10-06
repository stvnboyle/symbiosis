// Package awsx holds what every module needs to talk to AWS: loading credentials,
// checking the account, classifying errors and retrying.
package awsx

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// LoadConfig builds SDK configuration for one named profile and one region.
//
// Both are required. symbiosis never falls back to the default profile or to
// environment credentials, because picking up the wrong credentials silently is how
// tools end up changing the wrong account.
//
// No AWS call is made here. Credentials are resolved on first use.
func LoadConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	if profile == "" {
		return aws.Config{}, errors.New("no AWS profile given")
	}
	if region == "" {
		return aws.Config{}, errors.New("no AWS region given")
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	var missing config.SharedConfigProfileNotExistError
	if errors.As(err, &missing) {
		return aws.Config{}, fmt.Errorf("AWS profile %q is not defined in your AWS config: create it with `aws configure sso --profile %s`: %w", profile, profile, err)
	}
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config for profile %q: %w", profile, err)
	}
	return cfg, nil
}
