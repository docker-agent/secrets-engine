// Copyright 2025-2026 Docker, Inc.
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

package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/secrets-engine/plugins/pass/teststore"
)

func Test_StoreContext_roundtrip(t *testing.T) {
	t.Parallel()
	mock := teststore.NewMockStore()
	ctx := WithStore(t.Context(), mock)
	got, err := StoreFrom(ctx)
	require.NoError(t, err)
	assert.Same(t, mock, got)
}

func Test_StoreContext_missingReturnsError(t *testing.T) {
	t.Parallel()
	got, err := StoreFrom(context.Background())
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrNoStoreInContext)
}
