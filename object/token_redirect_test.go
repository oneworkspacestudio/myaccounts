// Copyright 2026 The One Workspace Authors.
// SPDX-License-Identifier: Apache-2.0

package object

import "testing"

func TestAuthorizationCodeRedirectRequiresExactOriginalRequest(t *testing.T) {
	bound := "https://chat.example.test/native/callback?device=one&mode=link"
	for _, test := range []struct {
		name, bound, supplied string
		valid                 bool
	}{
		{"exact", bound, bound, true},
		{"missing", bound, "", false},
		{"legacy-unbound", "", bound, false},
		{"both-missing", "", "", false},
		{"other-registered-callback", bound, "https://chat.example.test/other", false},
		{"host-case-is-not-normalized", bound, "https://CHAT.example.test/native/callback?device=one&mode=link", false},
		{"query-order-is-not-normalized", bound, "https://chat.example.test/native/callback?mode=link&device=one", false},
		{"changed-query", bound, "https://chat.example.test/native/callback?device=two&mode=link", false},
		{"native-exact", "uk.co.oneworkspace.chat:/oauth2redirect", "uk.co.oneworkspace.chat:/oauth2redirect", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validAuthorizationCodeRedirect(test.bound, test.supplied); got != test.valid {
				t.Fatalf("redirect binding valid=%v, want%v", got, test.valid)
			}
		})
	}
}
