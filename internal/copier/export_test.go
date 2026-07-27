package copier

// WithLsRemoteRunner exposes withLsRemoteRunner to black-box tests for
// stubbing git ls-remote invocations.
var WithLsRemoteRunner = withLsRemoteRunner

// LsRemoteRunner exposes the lsRemoteRunner function type to black-box tests.
type LsRemoteRunner = lsRemoteRunner
