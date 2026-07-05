package github

// WithGHRunner exposes withGHRunner to black-box tests for stubbing gh CLI calls.
var WithGHRunner = withGHRunner

// GHRunner exposes the ghRunner function type to black-box tests.
type GHRunner = ghRunner

// ParseChecks exposes the unexported parseChecks helper to black-box tests.
var ParseChecks = parseChecks

// StatusCheck exposes the unexported statusCheck type to black-box tests.
type StatusCheck = statusCheck
