Docker Pass is a helper for securely storing secrets in your local OS keychain and injecting them into containers when needed.
It uses platform-specific credential storage:

  - Windows: Windows Credential Manager API
  - macOS:   Keychain services API
  - Linux:   `org.freedesktop.secrets` API (requires DBus + `gnome-keyring` or `kdewallet`)

Secrets can be injected into running containers at runtime using the `se://` URI scheme.
