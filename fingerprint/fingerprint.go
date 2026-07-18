// Package fingerprint computes stable project cache keys.
package fingerprint

import (
	"github.com/pawnkit/pawnkit-core/hash"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

// Inputs contains the state used to build a fingerprint.
type Inputs struct {
	Manifest  *manifest.Manifest
	Lock      *lockfile.Lock
	ProfileID string
}

// Compute returns a stable "sha256:<hex>" fingerprint for in.
func Compute(in Inputs) (string, error) {
	manifestHash, err := hash.JSON(in.Manifest)
	if err != nil {
		return "", err
	}

	lockHash, err := hash.JSON(in.Lock)
	if err != nil {
		return "", err
	}

	return hash.Combine(manifestHash, lockHash, in.ProfileID), nil
}
