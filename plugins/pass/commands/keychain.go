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
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/secrets-engine/store/keychain"
)

const lockedKeychainHint = "Unlock the keychain and retry: log in to the desktop session, " +
	"or run gnome-keyring-daemon --unlock on a headless host."

const duplicateItemHint = "Use --force to overwrite the existing secret."

func wrapKeychainErrors(cmd *cobra.Command) *cobra.Command {
	if pre := cmd.PreRunE; pre != nil {
		cmd.PreRunE = func(c *cobra.Command, args []string) error {
			return withKeychainHint(pre(c, args))
		}
	}
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return withKeychainHint(run(c, args))
		}
	}
	return cmd
}

func withKeychainHint(err error) error {
	if errors.Is(err, keychain.ErrCollectionLocked) {
		return fmt.Errorf("%w\n\n%s", err, lockedKeychainHint)
	}
	if errors.Is(err, keychain.ErrDuplicateItem) {
		return fmt.Errorf("%w\n\n%s", err, duplicateItemHint)
	}
	return err
}
