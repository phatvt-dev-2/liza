package commands

import (
	"github.com/liza-mas/liza/internal/ops"
)

// ClearStaleReviewClaimsCommand finds and clears expired review leases on REVIEWING tasks.
// Delegates to ops.ClearStaleReviewClaims.
func ClearStaleReviewClaimsCommand(projectRoot string) (int, error) {
	return ops.ClearStaleReviewClaims(projectRoot)
}
