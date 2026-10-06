package awsx

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// fakeSTS returns a fixed identity and counts calls.
type fakeSTS struct {
	out   *sts.GetCallerIdentityOutput
	err   error
	calls int
}

func (f *fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	f.calls++
	return f.out, f.err
}

func identityOutput(account string) *sts.GetCallerIdentityOutput {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String(account),
		Arn:     aws.String("arn:aws:sts::" + account + ":assumed-role/Admin/steven"),
		UserId:  aws.String("AROAEXAMPLE:steven"),
	}
}

func TestPinCallerIdentity(t *testing.T) {
	api := &fakeSTS{out: identityOutput("123456789012")}
	got, err := CallerIdentity(context.Background(), api)
	if err != nil {
		t.Fatal(err)
	}
	want := Identity{
		Account: "123456789012",
		ARN:     "arn:aws:sts::123456789012:assumed-role/Admin/steven",
		UserID:  "AROAEXAMPLE:steven",
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPinCallerIdentityError(t *testing.T) {
	boom := errors.New("ExpiredToken")
	_, err := CallerIdentity(context.Background(), &fakeSTS{err: boom})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
}

func TestPinCallerIdentityWithoutAccountIsAnError(t *testing.T) {
	_, err := CallerIdentity(context.Background(), &fakeSTS{out: &sts.GetCallerIdentityOutput{}})
	if err == nil {
		t.Fatal("accepted an identity with no account id")
	}
}

func TestPinCheck(t *testing.T) {
	id := Identity{Account: "123456789012"}
	if err := CheckPin("123456789012", id); err != nil {
		t.Errorf("matching account rejected: %v", err)
	}

	err := CheckPin("999999999999", id)
	if !errors.Is(err, ErrWrongAccount) {
		t.Fatalf("error = %v, want ErrWrongAccount", err)
	}
	for _, want := range []string{"123456789012", "999999999999"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name account %s", err, want)
		}
	}
}

func TestPinCheckRefusesAnEmptyPin(t *testing.T) {
	if err := CheckPin("", Identity{Account: "123456789012"}); !errors.Is(err, ErrWrongAccount) {
		t.Errorf("error = %v, want ErrWrongAccount: an empty pin must never match", err)
	}
}

func TestPinVerifyAccount(t *testing.T) {
	api := &fakeSTS{out: identityOutput("123456789012")}
	id, err := VerifyAccount(context.Background(), api, "123456789012")
	if err != nil {
		t.Fatal(err)
	}
	if id.Account != "123456789012" || api.calls != 1 {
		t.Errorf("identity %+v after %d calls, want the account after exactly 1", id, api.calls)
	}

	// On a mismatch the identity is still returned, so callers can report it.
	id, err = VerifyAccount(context.Background(), api, "999999999999")
	if !errors.Is(err, ErrWrongAccount) {
		t.Fatalf("error = %v, want ErrWrongAccount", err)
	}
	if id.Account != "123456789012" {
		t.Errorf("identity not returned alongside the mismatch: %+v", id)
	}
}
