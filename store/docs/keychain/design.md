# Keyring

The `keyring` package is a convenient cross-platform library that supports
storing any data inside the OS keychain.

## How does this compare to the [docker-credential-helpers](https://github.com/docker/docker-credential-helpers/)?

It achieves a similar goal, but the main distinction is in its purpose.
The `docker-credential-helpers` were created with the explicit intent of storing
remote service credentials, such as Image Registry credentials. Each value is
linked to a hostname instead of a generic key.

The `keyring` package is tightly coupled to the secrets engine `secrets.ID` which
allows for more generic identifiers but in a strict format. A key can be specified
as `key=realm/group/application/username`.

The drawbacks of the credential helper is that it is shipped as a separate binary.
In the past this was necessary, so that external applications could access Docker
specific credentials. It has, however, caused a lot of unexpected headache.

Many users face the problem that the credential helper binary goes missing from
the system PATH or the docker config file gets altered, breaking Docker Desktop
or the Docker CLI from properly fetching login credentials.

With the Secrets Engine the need for shipping a separate binary becomes unnecessary
and moving the credential helper into a library would reduce the burden users
are experiencing.

## Linux

Users running Linux with a desktop environment usually have access to the
[`org.freedesktop.secrets`](https://specifications.freedesktop.org/secret-service-spec/latest/index.html) API
via `gnome-keyring` or `kdewallet`.

Usually the `pam_gnome_keyring.so` and `pam_kwallet5.so` would hook into PAM
and automatically unlock the 'login' keyring once the user does a login to their system.
If the 'login' keyring does not exist, it will be created using the user's login password.
If the 'login' keyring is the first keyring created, it will be set as the default.
For more information regarding `gnome-keyring` and PAM, please refer to the
[GnomeKeyring documentation](https://wiki.gnome.org/Projects/GnomeKeyring/Pam)

In the `keyring_linux.go` file, we attempt to use the `login` keyring or in terms
of dbus terminology the `login collection`. If no such keyring can be found,
it defaults finding the default keyring.

To communicate with the `org.freedesktop.secrets` API, we are using `dbus`.
It is a convenient way of communicating without needing any direct C library integration.

### Eager availability check

`New` eagerly verifies that the keychain backend is reachable before returning,
so callers can detect an unusable host (for example WSL or a headless machine
with no D-Bus session bus, or a desktop with no `gnome-keyring`/`kwallet`
running) at construction time and fall back gracefully:

```go
st, err := keychain.New(ctx, group, name, factory)
if errors.Is(err, keychain.ErrKeychainUnavailable) {
    // backend unreachable on this host — fall back to another store
}
```

`New` takes a `context.Context`. On Linux it bounds (and can cancel) the probe's
D-Bus connection handshake and its `NameHasOwner` round-trip. When `ctx` carries
no deadline, `New` caps the probe with a short internal default
(`defaultProbeTimeout`, 2s) so construction stays responsive on an unreachable
host; a caller-supplied deadline always wins. On macOS/Windows the probe is a
no-op and `ctx` is unused. `New` does not retain `ctx`: it governs construction
only, not later store operations.

On Linux the check dials a fresh connection through the same path every
operation uses and asks the **D-Bus daemon** whether `org.freedesktop.secrets`
has an owner, via `org.freedesktop.DBus.NameHasOwner`. It is intentionally
**prompt-safe and side-effect-free**: the query is answered by `dbus-daemon` from
its own name registry and is never forwarded to the Secret Service backend, so it
can never reach a password/unlock prompt, never activates the backend, and never
touches a collection or item. The probe connection is closed immediately,
preserving the fresh-connection-per-operation design.

The dial itself never launches a session bus: `NewService` uses
`dbus.SessionBusPrivateNoAutoStartup` rather than `dbus.ConnectSessionBus`, so on
a host with no running bus it fails fast instead of spawning `dbus-launch` (which
would be both slow and a side effect). As an additional fast path the dial stats
the session bus unix socket first and gives up immediately if it is missing. The
raw socket dial performed by `godbus` is not itself `ctx`-aware, but a unix
socket dial fails immediately when the socket is missing or unaccepting, and the
`ctx` bounds everything from the authentication handshake onward.

`ErrKeychainUnavailable` is the caller's fallback signal and is **distinct from**
`ErrNoDefaultCollection`: the availability check does not assert that a
collection exists, so a reachable-but-uninitialized keyring still passes `New`
and surfaces `ErrNoDefaultCollection` lazily on the first operation, as before.
On macOS and Windows the check is a no-op (`New` never returns
`ErrKeychainUnavailable` there).

### Locked collections and the bounded unlock prompt

A reachable backend can still hold a **locked** collection — the default state
on headless hosts with SSH key-only logins, where PAM has no password to
auto-unlock the login keyring with, so it relocks on every keyring-daemon
restart.

Every store operation checks the collection's lock state up front
(`ensureCollectionUnlocked`) and, when locked, issues a Secret Service
`Unlock`. On a passwordless keyring that completes silently via the null
prompt. On a password-protected keyring it opens the backend's unlock prompt,
and that prompt wait is **bounded twice**:

- by the operation's own `ctx` (deliberately the caller's original context,
  not the `context.WithoutCancel` connection context: in-flight D-Bus calls
  are protected from teardown, but waiting on a human is bounded by the
  caller); and
- by an internal 30s cap (`promptTimeout`), so a prompt nobody can ever answer
  cannot block an operation forever even without a caller deadline.

When the unlock fails — prompt dismissed, timed out, or ctx expired — the
operation fails with an error wrapping the exported `ErrCollectionLocked`
sentinel and naming the collection, with the underlying prompt failure
preserved as the cause. The same wrapping applies inside the relock-retry loop
(`withRelockRetry`) and when a collection is still locked after the bounded
retries.

Empirically (validated live on Ubuntu 24.04, gnome-keyring): on a headless
host the unlock prompt does not hang — gnome-keyring completes it as
*dismissed* within milliseconds when no prompter can be shown (whether or not
the gcr prompter is installed and D-Bus-activatable), so the locked error
surfaces in ~15ms. The 30s cap and ctx bound cover the remaining case of a
live prompter with an absent user.

Deliberately **not** built (see the decision log): prompter-presence probes
(`org.gnome.keyring.SystemPrompter` is activatable-but-unstartable on headless
hosts with gcr installed, and KWallet/KeePassXC never own that name — both
directions misclassify), password callbacks, programmatic master-password
unlock (`org.gnome.keyring.InternalUnsupportedGuiltRiddenInterface`), and
library-owned TTY prompting. The library reports the locked state reliably;
the caller owns remediation. `ErrCollectionLocked` also must not be treated as
"unavailable, fall back": the locked collection still holds the user's
credentials, and silently writing new ones to a fallback store would split
credentials across two stores.
