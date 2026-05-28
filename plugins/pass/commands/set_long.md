Stores a secret in the local OS keychain. The secret value can be provided inline (`NAME=VALUE`) or piped via STDIN.

Behavior when a secret with the same id already exists is platform-dependent:
  - macOS (Keychain): the command fails with a duplicate-item error.
  - Linux (Secret Service) and Windows (Credential Manager): the existing
    value is silently overwritten.

Pass `--force` to overwrite an existing secret. On Linux and Windows the
replacement is performed atomically. On macOS the Keychain API requires
a delete-then-add sequence.
