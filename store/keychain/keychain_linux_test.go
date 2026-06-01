// Copyright 2026 Docker, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package keychain

import (
	"os"
	"testing"

	dbus "github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/secrets-engine/store"
)

// openFDCount returns the number of open file descriptors held by the current
// process by counting entries in /proc/self/fd (Linux only).
func openFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	return len(entries)
}

// TestKeychainDoesNotLeakConnections is a regression test for a D-Bus
// connection leak: each keychain operation dialed a fresh session-bus
// connection (kc.NewService -> dbus.ConnectSessionBus) but only closed the
// secret-service session, never the connection itself. Every operation
// therefore leaked one socket file descriptor, eventually exhausting the
// session bus's max_connections_per_user limit on long-lived processes.
//
// It reproduces the failure shape directly: perform many lookups and assert
// the process's open-fd count stays bounded instead of growing
// one-per-operation.
//
// The lookups target a non-existent id so the test exercises the full
// connection setup (NewService -> OpenSession -> resolve collection ->
// SearchCollection) and then returns ErrCredentialNotFound *before* fetching a
// secret. That keeps the test focused on the connection lifecycle — the thing
// the fix changes — without creating, reading, or deleting shared keyring
// items (which is both stateful and prone to gnome-keyring item-lock quirks).
func TestKeychainDoesNotLeakConnections(t *testing.T) {
	ks := setupKeychain(t, nil)
	missing := store.MustParseID("com.test.test/test/does-not-exist")

	const iterations = 30

	get := func() {
		_, err := ks.Get(t.Context(), missing)
		require.ErrorIs(t, err, store.ErrCredentialNotFound)
	}

	// Warm up so any one-time, non-leaking fds (lazy runtime/dbus init) are
	// already open before we take the baseline.
	for range 3 {
		get()
	}

	before := openFDCount(t)
	for range iterations {
		get()
	}
	after := openFDCount(t)

	// A correct implementation closes every connection it opens, so the fd
	// count should be flat. Allow a small slack for unrelated runtime churn;
	// the leak grows the count by ~iterations, far above the threshold.
	const slack = 5
	assert.LessOrEqualf(t, after-before, slack,
		"open fd count grew by %d over %d lookups (before=%d after=%d): D-Bus connections are leaking",
		after-before, iterations, before, after)
}

func TestResolveDefaultCollection(t *testing.T) {
	const customCollection = dbus.ObjectPath("/org/freedesktop/secrets/collection/custom")

	tests := []struct {
		name        string
		collections []dbus.ObjectPath
		aliasPath   dbus.ObjectPath
		want        dbus.ObjectPath
		wantErr     error
	}{
		{
			name:        "prefers login collection when present",
			collections: []dbus.ObjectPath{customCollection, loginKeychainObjectPath},
			// even if the alias points elsewhere, login wins
			aliasPath: customCollection,
			want:      loginKeychainObjectPath,
		},
		{
			name:        "falls back to default alias when login is absent",
			collections: []dbus.ObjectPath{customCollection},
			aliasPath:   customCollection,
			want:        customCollection,
		},
		{
			name: "rejects null path from unassigned default alias",
			// headless host: no login collection, no default alias set, so
			// ReadAlias returns the null object path "/"
			collections: []dbus.ObjectPath{},
			aliasPath:   nullObjectPath,
			wantErr:     ErrNoDefaultCollection,
		},
		{
			name:        "rejects syntactically invalid alias path",
			collections: []dbus.ObjectPath{},
			aliasPath:   dbus.ObjectPath(""),
			// distinct from the null-path case; see resolveDefaultCollection
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDefaultCollection(tt.collections, tt.aliasPath)

			switch {
			case tt.wantErr != nil:
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
			case tt.want == "":
				// invalid path case: an error is expected, value is empty
				require.Error(t, err)
				assert.Empty(t, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
