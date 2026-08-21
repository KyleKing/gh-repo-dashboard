package copier

import (
	"time"

	"github.com/kyleking/aragonite/cache"
)

const latestTagTTL = 30 * time.Minute

// LatestTagCache is keyed by a template's _src_path rather than by repo path,
// so every repo generated from the same upstream template shares one lookup
// instead of each repo hitting the network on its own.
//
//nolint:gochecknoglobals // one process-wide cache, the point of registering it
var LatestTagCache = cache.NewRegistered[string](latestTagTTL)
