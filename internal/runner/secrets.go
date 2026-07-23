package runner

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"keepersecurity.com/ksm-scim/scim"
)

// SecretProviderEnv selects where the SCIM sync parameters come from. Defaults
// to "KSM" (a Keeper Secrets Manager record) when unset; "GCP" builds them from
// per-field Google Secret Manager secrets.
const SecretProviderEnv = "SECRET_PROVIDER"

const (
	providerKSM = "KSM"
	providerGCP = "GCP"
)

// Env vars naming the Google Secret Manager secret for each SCIM parameter when
// SECRET_PROVIDER=GCP. Each value is a secret reference: a short secret name
// (project auto-detected via the metadata server, version "latest") or a full
// "projects/{project}/secrets/{secret}/versions/{version}" resource path.
const (
	GCPScimURLEnv      = "SCIM_URL"
	GCPScimTokenEnv    = "SCIM_TOKEN"
	GCPAdminAccountEnv = "GCP_ADMIN_ACCOUNT"
	GCPCredentialsEnv  = "GCP_CREDENTIALS"
	GCPScimGroupsEnv   = "SCIM_GROUPS"
)

// Behavioral flags for the GCP provider. These are read as plain env-var values
// (not secrets), matching the optional "Verbose"/"Destructive" record fields.
const (
	GCPVerboseEnv     = "SCIM_VERBOSE"
	GCPDestructiveEnv = "SCIM_DESTRUCTIVE"
)

// loadScimParametersFromGCP builds the SCIM sync parameters from per-field
// Google Secret Manager secrets named by the GCP* env vars. A single Secret
// Manager client is reused for every field.
func loadScimParametersFromGCP(ctx context.Context) (ka *scim.ScimEndpointParameters, gcp *scim.GoogleEndpointParameters, err error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("creating Secret Manager client: %w", err)
	}
	defer client.Close()

	get := func(envVarName string) (string, error) {
		ref := os.Getenv(envVarName)
		if len(ref) == 0 {
			return "", fmt.Errorf("environment variable %q is not set", envVarName)
		}
		return accessSecret(ctx, client, ref)
	}

	url, err := get(GCPScimURLEnv)
	if err != nil {
		return nil, nil, err
	}
	token, err := get(GCPScimTokenEnv)
	if err != nil {
		return nil, nil, err
	}
	adminAccount, err := get(GCPAdminAccountEnv)
	if err != nil {
		return nil, nil, err
	}
	credentials, err := get(GCPCredentialsEnv)
	if err != nil {
		return nil, nil, err
	}
	groupsRaw, err := get(GCPScimGroupsEnv)
	if err != nil {
		return nil, nil, err
	}

	scimGroups := parseScimGroupList(groupsRaw)
	if len(scimGroups) == 0 {
		return nil, nil, fmt.Errorf("secret referenced by %q contains no SCIM groups", GCPScimGroupsEnv)
	}

	gcp = &scim.GoogleEndpointParameters{
		AdminAccount: adminAccount,
		Credentials:  []byte(credentials),
		ScimGroups:   scimGroups,
	}
	ka = &scim.ScimEndpointParameters{
		Url:   url,
		Token: token,
	}

	if v := os.Getenv(GCPVerboseEnv); len(v) > 0 {
		verbose, er := strconv.ParseBool(v)
		if er != nil {
			return nil, nil, fmt.Errorf("invalid %q value %q: %w", GCPVerboseEnv, v, er)
		}
		ka.Verbose = verbose
	}
	if v := os.Getenv(GCPDestructiveEnv); len(v) > 0 {
		destructive, er := strconv.Atoi(v)
		if er != nil {
			return nil, nil, fmt.Errorf("invalid %q value %q: %w", GCPDestructiveEnv, v, er)
		}
		ka.Destructive = int32(destructive)
	}

	return ka, gcp, nil
}

// parseScimGroupList splits a secret payload into SCIM group names. Groups may
// be separated by newlines or commas; surrounding whitespace and empty entries
// are dropped.
func parseScimGroupList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	var groups []string
	for _, f := range fields {
		if g := strings.TrimSpace(f); len(g) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

// accessSecret fetches the payload of a Google Secret Manager reference using
// the given client. ref may be a short secret name or a full resource path (see
// gcpSecretResourceName).
func accessSecret(ctx context.Context, client *secretmanager.Client, ref string) (string, error) {
	name, err := gcpSecretResourceName(ctx, ref)
	if err != nil {
		return "", err
	}

	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("accessing secret %q: %w", name, err)
	}

	return string(result.GetPayload().GetData()), nil
}

// gcpSecretResourceName turns a secret reference into a fully-qualified Secret
// Manager version resource name.
//
// ref may be either a short secret name (e.g. "scim-token"), in which case the
// GCP project is auto-detected via the metadata server and version "latest" is
// used, or a full resource path
// ("projects/{project}/secrets/{secret}/versions/{version}"). A resource path
// without a "/versions/" segment defaults to version "latest".
func gcpSecretResourceName(ctx context.Context, ref string) (string, error) {
	if strings.HasPrefix(ref, "projects/") {
		if strings.Contains(ref, "/versions/") {
			return ref, nil
		}
		return ref + "/versions/latest", nil
	}

	project, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("auto-detecting GCP project for secret %q (set a full projects/.../secrets/... reference to avoid metadata lookup): %w", ref, err)
	}
	return fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, ref), nil
}
