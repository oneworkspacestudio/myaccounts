// Copyright 2026 The One Workspace Authors.
// SPDX-License-Identifier: Apache-2.0

package object

// Do not normalize, pattern-match against the application's other callbacks,
// or infer the binding for codes issued by an older server. A new authorization
// request can reuse a valid provider session without weakening this boundary.
func validAuthorizationCodeRedirect(bound, supplied string) bool {
	return bound != "" && supplied != "" && bound == supplied
}
