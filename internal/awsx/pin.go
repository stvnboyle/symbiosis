package awsx

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// ErrWrongAccount means the credentials in use belong to a different AWS account
// from the one symbiosis is pinned to.
var ErrWrongAccount = errors.New("wrong AWS account")

// Identity is who AWS says the current credentials belong to.
type Identity struct {
	Account string
	ARN     string
	UserID  string
}

// stsAPI is the slice of STS that identity checks need.
type stsAPI interface {
	GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// CallerIdentity asks AWS who the current credentials belong to.
//
// GetCallerIdentity needs no IAM permission and cannot be denied by policy, which
// makes it the one call that is always safe to make first.
func CallerIdentity(ctx context.Context, api stsAPI) (Identity, error) {
	out, err := api.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return Identity{}, fmt.Errorf("get caller identity: %w", err)
	}
	id := Identity{
		Account: aws.ToString(out.Account),
		ARN:     aws.ToString(out.Arn),
		UserID:  aws.ToString(out.UserId),
	}
	if id.Account == "" {
		return Identity{}, errors.New("get caller identity: AWS returned no account id")
	}
	return id, nil
}

// CheckPin returns ErrWrongAccount unless id is in the pinned account.
func CheckPin(pinned string, id Identity) error {
	if pinned == "" || pinned != id.Account {
		return fmt.Errorf("%w: these credentials are for account %s, but symbiosis is pinned to %q", ErrWrongAccount, id.Account, pinned)
	}
	return nil
}

// VerifyAccount fetches the caller's identity and checks it against the pinned
// account. Every command calls this before any other AWS call. On a mismatch it
// returns the identity alongside ErrWrongAccount.
func VerifyAccount(ctx context.Context, api stsAPI, pinned string) (Identity, error) {
	id, err := CallerIdentity(ctx, api)
	if err != nil {
		return Identity{}, err
	}
	return id, CheckPin(pinned, id)
}
