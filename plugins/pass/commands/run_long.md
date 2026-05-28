Scans the current environment (plus any `--env-file` inputs) for variables
whose value is exactly `se://<ID|pattern>`. Each reference is resolved through the
secrets-engine daemon and the resolved value is passed to the child process.
The child inherits stdin, stdout, and stderr.

Requires the secrets-engine daemon (Docker Desktop) to be running.

If any reference cannot be resolved, the command fails before the child is
started and exits non-zero.
