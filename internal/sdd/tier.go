package sdd

import "strings"

// Tier is the ceremony weight of a task: how much design gate and review a change
// deserves, so the loop scales effort to risk instead of treating a placeholder
// screen like a payment flow.
type Tier string

const (
	// TierTrivial — a copy tweak, a rename, a screen composing only existing base
	// components. Skips the design gate; a single, shallow review; no cross-file
	// refactors under review.
	TierTrivial Tier = "trivial"
	// TierStandard — the default: a normal feature with some logic. Full review,
	// but the second round runs only if the first changed code.
	TierStandard Tier = "standard"
	// TierCritical — money, auth, or data mutation. Full ceremony, both review
	// rounds always, security lens mandatory.
	TierCritical Tier = "critical"
)

// criticalKeywords are strong, unambiguous risk signals. Kept conservative on
// purpose: inference should escalate only when the signal is clear, and the
// override handles anything it misses. Role-ish words (caja, rol) are excluded
// because they appear in low-risk routing/UI tasks too.
var criticalKeywords = []string{
	"pago", "pagos", "cobro", "cobrar", "checkout", "payment", "billing",
	"factura", "boleta", "invoice", "precio",
	"auth", "login", "signin", "sign-in", "token", "password", "contraseña",
	"credencial", "credential", "session", "sesion", "sesión", "oauth",
	"security", "seguridad",
	"migration", "migracion", "migración", "schema", "backup", "encrypt",
	"secret", "delete", "drop",
}

// trivialKeywords are weak signals — trivial is best set via the override; these
// only catch the obvious chores.
var trivialKeywords = []string{
	"typo", "rename", "renombrar", "chore", "docs", "placeholder", "alias", "comment",
}

// ResolveTier returns a task's tier: the frontmatter `tier:` override when set to
// a valid value, otherwise inferred from the task's title and tags. It never
// silently under-rates — a critical signal wins over a trivial one.
func ResolveTier(taskFM map[string]string) Tier {
	if t, ok := parseTier(taskFM["tier"]); ok {
		return t
	}
	return InferTier(taskFM["title"] + " " + taskFM["tags"])
}

// InferTier classifies free text by keyword. Critical is checked first so a
// money/auth/data signal is never masked by a trivial one; trivial only applies
// when there is no critical signal.
func InferTier(text string) Tier {
	blob := strings.ToLower(text)
	for _, kw := range criticalKeywords {
		if strings.Contains(blob, kw) {
			return TierCritical
		}
	}
	for _, kw := range trivialKeywords {
		if strings.Contains(blob, kw) {
			return TierTrivial
		}
	}
	return TierStandard
}

// parseTier validates a raw frontmatter/flag tier value. Empty or unknown values
// report ok=false so the caller falls back to inference.
func parseTier(raw string) (Tier, bool) {
	switch Tier(strings.ToLower(strings.TrimSpace(raw))) {
	case TierTrivial:
		return TierTrivial, true
	case TierStandard:
		return TierStandard, true
	case TierCritical:
		return TierCritical, true
	}
	return "", false
}
