package auth

import (
	"context"
	"fmt"
	"time"
)

// RefreshRotationService builds the pattern production OAuth2 servers
// adopted after tokens lived long enough to be stolen: every login
// starts a FAMILY; every exchange CONSUMES the presented refresh token
// (one-time use) and issues the next link in the same family; and a
// presented token that is ALREADY used is not an error to forgive — it
// is theft in progress. Two honest parties never present the same
// refresh token twice, so the second presentation means a copy exists
// in someone else's hands, and the only safe response is to kill the
// ENTIRE family — every token in the chain, current and future — and
// force a fresh login. Tokens are opaque high-entropy handles stored
// as SHA-256 hashes; the raw value rides exactly one response.
type RefreshRotationService struct {
	store     RefreshStore
	directory UserDirectory
	passwords *Hasher
	tokens    *TokenService
}

// NewRefreshRotationService wires the engine over its ports.
func NewRefreshRotationService(store RefreshStore, directory UserDirectory, passwords *Hasher, tokens *TokenService) *RefreshRotationService {
	return &RefreshRotationService{store: store, directory: directory, passwords: passwords, tokens: tokens}
}

// RefreshLoginResult is what a login answers: the family id, the raw
// FIRST refresh token (this response and nowhere else), and the access
// token it travels with.
type RefreshLoginResult struct {
	FamilyID     string
	Identity     Identity
	AccessToken  string
	RefreshToken string
}

// Login verifies credentials and starts family #1: one refresh token
// at position 1, plus an access token for the road.
func (s *RefreshRotationService) Login(ctx context.Context, username, password string, now time.Time) (RefreshLoginResult, error) {
	user, _, err := s.directory.FindUser(ctx, username)
	if err != nil {
		return RefreshLoginResult{}, fmt.Errorf("find user %q: %w", username, err)
	}
	// Empty hash on an unknown user burns the decoy — see SessionService.
	if !s.passwords.VerifyUniform(password, user.PasswordHash) {
		return RefreshLoginResult{}, ErrInvalidCredentials
	}
	familyID, err := randomToken(9)
	if err != nil {
		return RefreshLoginResult{}, err
	}
	familyID = "fam_" + familyID
	refresh, err := s.refreshToken()
	if err != nil {
		return RefreshLoginResult{}, err
	}
	if err := s.store.StartFamily(ctx, StoredRefreshFamily{ID: familyID, Username: user.Username, Status: "active"},
		StoredRefreshToken{
			SHA256:   SHA256Hex(refresh),
			FamilyID: familyID,
			Position: 1,
			Status:   "active",
			IssuedAt: now,
		}); err != nil {
		return RefreshLoginResult{}, fmt.Errorf("start refresh family: %w", err)
	}
	minted, err := s.tokens.Mint(MintTokenRequest{Subject: user.Username, Scopes: user.Scopes, TTL: AccessTokenTTL, Now: now})
	if err != nil {
		return RefreshLoginResult{}, err
	}
	return RefreshLoginResult{
		FamilyID:     familyID,
		Identity:     Identity{Username: user.Username, DisplayName: user.DisplayName, Scopes: user.Scopes, Via: "refresh"},
		AccessToken:  minted.Token,
		RefreshToken: refresh,
	}, nil
}

// ExchangeOutcome labels the three ways an exchange can end.
const (
	RefreshRotated       = "rotated"
	RefreshReuseDetected = "reuse-detected"
)

// RefreshExchangeResult carries whichever ending happened: on
// rotation, the next token and a fresh access token; on reuse, the
// kill count as proof the alarm fired.
type RefreshExchangeResult struct {
	Outcome       string
	FamilyID      string
	AccessToken   string
	RefreshToken  string // the NEXT token, on rotation
	RotatedFrom   int
	FamilyRevoked int // tokens killed by the reuse alarm
}

// Exchange presents one refresh token: rotate it, or die loudly on
// reuse. Three store-backed outcomes: ACTIVE token → mark used, mint
// the next link; USED token → REUSE — revoke the whole family and
// answer the kill count; anything else (unknown hash, revoked family,
// expired lease) → the typed invalid-refresh refusal.
func (s *RefreshRotationService) Exchange(ctx context.Context, presented string, now time.Time) (RefreshExchangeResult, error) {
	presentedSHA := SHA256Hex(presented)
	stored, found, err := s.store.FindRefreshToken(ctx, presentedSHA)
	if err != nil {
		return RefreshExchangeResult{}, fmt.Errorf("find refresh token: %w", err)
	}
	if !found {
		return RefreshExchangeResult{}, fmt.Errorf("%w: unknown handle (families are per-user; login mints one)", ErrInvalidRefresh)
	}
	family, _, famErr := s.store.FindFamily(ctx, stored.FamilyID)
	if famErr != nil {
		return RefreshExchangeResult{}, fmt.Errorf("find refresh family: %w", famErr)
	}
	if !foundFamilyActive(family) {
		return RefreshExchangeResult{}, fmt.Errorf("%w: this family was revoked after a reuse event; log in again to start a new one", ErrInvalidRefresh)
	}
	if !now.Before(stored.IssuedAt.Add(RefreshTTL)) {
		return RefreshExchangeResult{}, fmt.Errorf("%w: the 24h lease ran out; log in again", ErrInvalidRefresh)
	}
	if stored.Status == "used" {
		killed, err := s.store.RevokeFamily(ctx, stored.FamilyID)
		if err != nil {
			return RefreshExchangeResult{}, fmt.Errorf("revoke refresh family: %w", err)
		}
		return RefreshExchangeResult{
			Outcome:       RefreshReuseDetected,
			FamilyID:      stored.FamilyID,
			RotatedFrom:   stored.Position,
			FamilyRevoked: killed,
		}, nil
	}
	if stored.Status != "active" {
		return RefreshExchangeResult{}, fmt.Errorf("%w: token is %q, not presentable", ErrInvalidRefresh, stored.Status)
	}

	if err := s.store.MarkRefreshUsed(ctx, presentedSHA, now); err != nil {
		return RefreshExchangeResult{}, fmt.Errorf("mark refresh used: %w", err)
	}
	next, err := s.refreshToken()
	if err != nil {
		return RefreshExchangeResult{}, err
	}
	if err := s.store.AddRefreshToken(ctx, StoredRefreshToken{
		SHA256:   SHA256Hex(next),
		FamilyID: stored.FamilyID,
		Position: stored.Position + 1,
		Status:   "active",
		IssuedAt: now,
	}); err != nil {
		return RefreshExchangeResult{}, fmt.Errorf("add refresh token: %w", err)
	}
	scopes := []string{}
	if user, userOK, err := s.directory.FindUser(ctx, family.Username); err != nil {
		return RefreshExchangeResult{}, fmt.Errorf("find user %q: %w", family.Username, err)
	} else if userOK {
		scopes = user.Scopes
	}
	minted, err := s.tokens.Mint(MintTokenRequest{Subject: family.Username, Scopes: scopes, TTL: AccessTokenTTL, Now: now})
	if err != nil {
		return RefreshExchangeResult{}, err
	}
	return RefreshExchangeResult{
		Outcome:      RefreshRotated,
		FamilyID:     stored.FamilyID,
		AccessToken:  minted.Token,
		RefreshToken: next,
		RotatedFrom:  stored.Position,
	}, nil
}

func (s *RefreshRotationService) refreshToken() (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	return "rt_" + token, nil
}

// foundFamilyActive reports whether the family row exists and lives.
// A missing row after a found token is a store bug — treat it as
// revoked (fail closed).
func foundFamilyActive(family StoredRefreshFamily) bool {
	return family.ID != "" && family.Status == "active"
}
