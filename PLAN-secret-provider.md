# Plan: Google Secret Manager support in RunFromEnv()

## Goal
Let `KSM_CONFIG_BASE64` and `KSM_RECORD_UID` be sourced from Google Secret
Manager instead of literal env var values, for Cloud Run deployments that
don't want raw secrets in `.env.yaml` / revision config.

## Design (confirmed with user)
- New env var `SECRET_PROVIDER`, defaults to `KSM` (today's behavior — the two
  env vars are used as literal values, fully backward compatible).
- `SECRET_PROVIDER=GCP` switches interpretation: the *values* of
  `KSM_CONFIG_BASE64` and `KSM_RECORD_UID` are treated as Secret Manager
  secret references instead of literal values, and are resolved at runtime.
- Reference format: accept a short secret name (e.g. `ksm-config`) — resolve
  the GCP project automatically via ADC/metadata server (available on Cloud
  Run) and default to version `latest`. Also accept a full resource path
  (`projects/{project}/secrets/{secret}/versions/{version}`) if given, for
  flexibility.
- Applies uniformly to both config options — same resolution helper used for
  each.
- If `SECRET_PROVIDER=GCP` but `KSM_RECORD_UID` is unset/empty, skip
  resolution (nothing to look up) — it's optional either way.

## Implementation steps
1. **Dependency** — done: `cloud.google.com/go/secretmanager` added via
   `go get` (brought in `secretmanager/apiv1` + transitive upgrades to
   otel/grpc/genproto/x-net/x-crypto/gax-go). go.mod/go.sum already reflect
   this.
2. **New file** `internal/runner/secrets.go`:
   - `const SecretProviderEnv = "SECRET_PROVIDER"`
   - `resolveEnvSecret(ctx, envVarName string) (string, error)`:
     - raw := os.Getenv(envVarName); if empty, return "", nil (nothing to
       resolve, caller decides if that's an error).
     - provider := os.Getenv(SecretProviderEnv); default "KSM" if empty.
     - if provider == "KSM" (case-insensitive): return raw, nil (literal).
     - if provider == "GCP": treat `raw` as a secret reference and call
       `fetchGCPSecret(ctx, raw)`.
     - else: error — unknown SECRET_PROVIDER value.
   - `fetchGCPSecret(ctx, ref string) (string, error)`:
     - if `ref` starts with `projects/`: use as-is; append
       `/versions/latest` if it has no `/versions/` segment.
     - else: auto-detect project via `cloud.google.com/go/compute/metadata`
       and build `projects/{project}/secrets/{ref}/versions/latest`.
     - `secretmanager.NewClient(ctx)`, `client.AccessSecretVersion(...)`,
       return `string(result.Payload.Data)`, close client, wrap errors with
       context (secret ref/resource name).
3. **Update `RunFromEnv()`** in `internal/runner/runner.go`:
   - Resolve `KSM_CONFIG_BASE64` via `resolveEnvSecret`; keep the existing
     "not set" error if the resolved value is empty.
   - Resolve `KSM_RECORD_UID` via `resolveEnvSecret` (optional, no error if
     empty).
   - Pass both into the existing `RunFromConfig`.
4. **Docs**: update `README.md` (Cloud Run config section) and
   `.env.yaml.sample` to mention `SECRET_PROVIDER` and the GCP secret-name
   format; note the Cloud Run service account needs
   `roles/secretmanager.secretAccessor` on the referenced secrets.
5. **Build/verify**: `go build ./...`, `go vet ./...`. No unit tests exist
   yet for `runner` package — consider whether to add one for the provider
   switch logic (pure string/branch logic, no live GCP calls needed for the
   `KSM` path; the `GCP` path would need a fake/mock Secret Manager client or
   be left to manual/integration testing).

## Status
Paused mid-implementation to set up a VS Code dev container (no
IntelliSense/gopls currently working locally). Resume at step 2 above.
