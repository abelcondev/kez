package cli

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/abelcondev/kez/internal/config"
	"github.com/abelcondev/kez/internal/oauth"
	"github.com/abelcondev/kez/internal/providers/providerio"
)

// oauthLoginForProfile resolves the user's OAuth login for a provider ONCE and
// returns both a TokenResolver that authenticates model calls with it and the
// credential-store key it bound to. It returns (nil, "") when no login exists —
// keeping API-key users free of any per-request store lookups, since the resolver
// is only attached when a login is present at construction time.
//
// The returned key is the single source of truth for "which login is this
// provider using": callers pass it to providers.Options.OAuthLoginKey so the
// Codex chatgpt-account-id header reads its account from the exact login that
// issued the bearer token, instead of doing a second, independent lookup that
// could select a different login (a backend-rejected mismatch).
//
// Candidate login names (profile name, then a catalog-ID fallback, both gated on
// the profile having no own configured credential) come from the shared
// ProviderProfile.OAuthLoginCandidates so the runtime resolver, the Codex account
// resolver, and the onboarding presence check never diverge.
func oauthLoginForProfile(profile config.ProviderProfile) (providerio.TokenResolver, string) {
	candidates := profile.OAuthLoginCandidates()
	if len(candidates) == 0 {
		return nil, ""
	}
	store, err := oauth.NewStore(oauth.StoreOptions{})
	if err != nil {
		return nil, ""
	}
	_, key, ok := oauth.FirstStored(store, candidates)
	if !ok {
		// No login under any candidate (or unreadable/invalid keys) → API-key
		// auth, no resolver.
		return nil, ""
	}
	manager, err := oauth.NewManager(oauth.ManagerOptions{
		Store:      store,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		// Refreshing a token the user logged into (possibly a preset provider like
		// xAI) re-resolves that provider's OAuth config, which needs the preset.
		AllowPresets: true,
	})
	if err != nil {
		return nil, ""
	}
	resolver := func(ctx context.Context, forceRefresh bool) (string, string, bool, error) {
		var token string
		var rerr error
		if forceRefresh {
			token, rerr = manager.Handle401(ctx, key)
		} else {
			token, rerr = manager.GetFresh(ctx, key)
		}
		if errors.Is(rerr, oauth.ErrNoToken) {
			// The login was removed since construction → fall back to the API key.
			return "", "", false, nil
		}
		if rerr != nil {
			return "", "", false, rerr
		}
		return "Authorization", "Bearer " + token, true, nil
	}
	return resolver, key
}
