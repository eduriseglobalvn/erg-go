# Security CI Gates

## Local commands

Run the same security gates locally before merging sensitive backend changes:

```sh
make security
make security-gosec
make security-secrets
make security-vuln
```

Reports are written to `.gocache/security/`, which is ignored by git.

## CI policy

Pull requests and pushes run a dedicated security job:

- `gosec -severity medium -confidence medium ./...`
- `gitleaks detect --redact --config .gitleaks.toml`
- `govulncheck -scan=package ./...`

The job fails on unresolved medium/high gosec findings, high-confidence secret leaks, and reachable vulnerable dependency packages. Reports are uploaded as CI artifacts for review.

## False positives

Use `#nosec` only when the finding is confirmed false positive and the annotation is narrow. The annotation must include a short reason next to the code. Do not suppress broad files or packages.

## Secret leak response

If gitleaks catches a real secret:

1. Revoke or rotate the key in the provider console immediately.
2. Remove the secret from git history or replace it with a placeholder.
3. Update the deployment secret store with the new value.
4. Re-run `make security-secrets`.
5. Record the provider, affected environment, rotation time, and follow-up owner in Jira. Do not paste secret values into Jira.
