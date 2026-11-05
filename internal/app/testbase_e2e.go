//go:build e2e

package app

import "os"

// testBaseURL lets the e2e suite redirect every checker to a local mock.
func testBaseURL() string { return os.Getenv("NAMEARY_TEST_BASE_URL") }
