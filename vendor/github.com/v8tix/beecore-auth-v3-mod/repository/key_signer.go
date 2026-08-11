package repository

import (
	"context"

	"cloud.google.com/go/kms/apiv1/kmspb"
	gax "github.com/googleapis/gax-go/v2"
)

// KeySigner abstracts the two Google Cloud KMS operations JWT generation and
// verification need: GetPublicKey (verification) and AsymmetricSign (token
// generation). *kms.KeyManagementClient (cloud.google.com/go/kms/apiv1)
// satisfies this interface directly — its methods already match this exact
// signature — so production code needs no wrapper. Tests can substitute a
// fake backed by a local RSA key pair instead of requiring a live KMS
// connection.
type KeySigner interface {
	GetPublicKey(ctx context.Context, req *kmspb.GetPublicKeyRequest, opts ...gax.CallOption) (*kmspb.PublicKey, error)
	AsymmetricSign(ctx context.Context, req *kmspb.AsymmetricSignRequest, opts ...gax.CallOption) (*kmspb.AsymmetricSignResponse, error)
}
