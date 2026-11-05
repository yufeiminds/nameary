//go:build !e2e

package app

// testBaseURL is empty in production builds. The e2e build tag replaces it so
// the compiled CLI can talk to a local mock registry instead of the internet.
func testBaseURL() string { return "" }
