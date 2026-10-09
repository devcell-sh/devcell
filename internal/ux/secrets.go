package ux

import "fmt"

// FormatSecretsPhase renders the post-resolution detail for the Secrets phase
// row and signals whether it should be a warning. Partial failures (some
// resolved, some failed) produce a warning with a short tag prefix so the
// panel's short-note extraction can derive "N failed" for the completion
// summary.
func FormatSecretsPhase(count, failed int) (string, bool) {
	if failed > 0 {
		return fmt.Sprintf("%d failed — %d resolved, %d failed", failed, count, failed), true
	}
	return fmt.Sprintf("%d resolved", count), false
}
