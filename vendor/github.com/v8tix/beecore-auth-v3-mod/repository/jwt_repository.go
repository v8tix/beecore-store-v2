package repository

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/golang-jwt/jwt/v4"
	"github.com/v8tix/beecore-auth-v3-mod/model"
)

// publicKeyCache holds the RSA public key fetched from KMS, shared across all
// copies of a Repository value via the pointer in NewRepository. The key is
// static for the process lifetime (KMS key rotation is an explicit, versioned
// operation — see the config's kms.version field — not something that
// changes underneath a running process), so it's fetched from KMS at most
// once instead of on every single token verification.
type publicKeyCache struct {
	mu  sync.Mutex
	key *rsa.PublicKey
}

var (
	ErrFetchPublicKey = errors.New("failed to fetch public key")
	ErrParsePublicKey = errors.New("failed to parse public key")
	ErrComputeToken   = errors.New("failed to compute token signing string")
	ErrSigningToken   = errors.New("failed to sign token")
	ErrVerifyToken    = errors.New("failed to verify token signature")
	ErrDisabledUser   = errors.New("disabled user")
	ErrTokenRevoked   = errors.New("token is revoked")
)

func (r Repository) GenerateAuthenticationToken(email, password, app string, expirationDuration time.Duration) (string, error) {
	ctx := context.Background()

	user, err := r.findUserByEmailAndPassword(ctx, email, password)
	if err != nil {
		slog.Error("GenerateAuthenticationToken: Failed to find user", "error", err)
		return "", err
	}

	if !user.Enabled {
		slog.Error("GenerateAuthenticationToken: User disabled", "user_id", user.ID)
		return "", ErrDisabledUser
	}

	existing, err := r.FindTokenByUserIDAndScope(ctx, user.ID, AuthenticationToken)
	if err != nil && !errors.Is(err, ErrTokenNotFound) {
		return "", err
	}
	if isRevoked(existing) {
		return "", ErrTokenRevoked
	}

	newToken, err := r.issueAccessToken(ctx, user, app, expirationDuration)
	if err != nil {
		slog.Error("GenerateAuthenticationToken: Failed to generate token", "user_id", user.ID, "error", err)
		return "", err
	}

	if err := r.upsertToken(ctx, user.ID, newToken, string(AuthenticationToken), false, time.Now().UTC().Add(expirationDuration)); err != nil {
		return "", err
	}

	return newToken, nil
}

// issueAccessToken builds the JWT claims for user and signs a new access
// token via KMS. Shared by GenerateAuthenticationToken (password grant) and
// RefreshAuthenticationToken (refresh-token grant) — the two differ only in
// how the caller proved their identity, not in how the resulting token gets
// built and signed.
func (r Repository) issueAccessToken(ctx context.Context, user *model.User, app string, expirationDuration time.Duration) (string, error) {
	roles, err := r.findUserRoles(ctx, user.ID)
	if err != nil {
		return "", err
	}

	roleIDs := make([]string, len(roles))
	for i, role := range roles {
		roleIDs[i] = role.ID
	}

	now := time.Now().UTC()

	claims := model.CustomClaims{
		UserID: user.ID,
		Roles:  roleIDs,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expirationDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    app,
		},
	}

	return r.generateAuthenticationToken(ctx, claims)
}

// getPublicKey retrieves the public key from Google KMS for JWT validation,
// caching it after the first fetch — every incoming authenticated request
// calls this via VerifyToken, so an uncached fetch means a KMS network round
// trip on every single request.
func (r Repository) getPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	r.pubKeyCache.mu.Lock()
	defer r.pubKeyCache.mu.Unlock()

	if r.pubKeyCache.key != nil {
		return r.pubKeyCache.key, nil
	}

	resp, err := r.KmsClient.GetPublicKey(ctx, &kmspb.GetPublicKeyRequest{Name: r.KmsKey})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchPublicKey, err)
	}

	pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(resp.Pem))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParsePublicKey, err)
	}

	r.pubKeyCache.key = pubKey

	return pubKey, nil
}

// generateAuthenticationToken generates and signs a JWT using Google KMS
func (r Repository) generateAuthenticationToken(ctx context.Context, claims model.CustomClaims) (string, error) {

	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Compute the SHA256 digest of the token's signing string
	signingString, err := token.SigningString()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrComputeToken, err)
	}

	hash := sha256.Sum256([]byte(signingString))

	// Call KMS to sign the digest
	req := &kmspb.AsymmetricSignRequest{
		Name: r.KmsKey,
		Digest: &kmspb.Digest{
			Digest: &kmspb.Digest_Sha256{
				Sha256: hash[:],
			},
		},
	}

	resp, err := r.KmsClient.AsymmetricSign(ctx, req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSigningToken, err)
	}

	// Encode the signature to base64 (Google KMS returns a raw binary signature)
	signature := base64.RawURLEncoding.EncodeToString(resp.Signature)

	// Construct the final JWT manually
	signedToken := fmt.Sprintf("%s.%s", signingString, signature)

	return signedToken, nil
}

// VerifyToken verifies a JWT token signature using Google KMS public key
func (r Repository) VerifyToken(ctx context.Context, tokenString string) (*model.CustomClaims, error) {
	// Get public key from KMS
	pubKey, err := r.getPublicKey(ctx)
	if err != nil {
		return nil, err
	}

	// Split the token into its parts: header.payload.signature
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: invalid JWT format", ErrVerifyToken)
	}

	signingString := fmt.Sprintf("%s.%s", parts[0], parts[1])

	// Decode base64 URL-safe signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode signature", ErrVerifyToken)
	}

	// Compute the SHA256 hash of the signing string
	hash := sha256.Sum256([]byte(signingString))

	// Verify the signature using RSA public key
	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], signature)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature", ErrVerifyToken)
	}

	// Parse the token and extract claims
	token, err := jwt.ParseWithClaims(tokenString, &model.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return pubKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse token", ErrVerifyToken)
	}

	// Ensure the token is valid
	if !token.Valid {
		return nil, fmt.Errorf("%w: token is invalid", ErrVerifyToken)
	}

	// Extract claims
	claims, ok := token.Claims.(*model.CustomClaims)
	if !ok {
		return nil, fmt.Errorf("%w: failed to extract claims", ErrVerifyToken)
	}

	return claims, nil
}
