package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	ksm "github.com/keeper-security/secrets-manager-go/core"
	"keepersecurity.com/ksm-scim/scim"
)

const KSMConfigEnv = "KSM_CONFIG_BASE64"
const KSMRecordUIDEnv = "KSM_RECORD_UID"

// RunFromEnv resolves the SCIM sync parameters according to SECRET_PROVIDER and
// runs the sync. With KSM (the default) the parameters come from a Keeper
// Secrets Manager record; with GCP they are built from per-field Google Secret
// Manager secrets.
func RunFromEnv() (*scim.SyncStat, error) {
	ctx := context.Background()

	provider := os.Getenv(SecretProviderEnv)
	if len(provider) == 0 {
		provider = providerKSM
	}

	switch strings.ToUpper(provider) {
	case providerKSM:
		configBase64 := os.Getenv(KSMConfigEnv)
		if len(configBase64) == 0 {
			return nil, fmt.Errorf("environment variable %q is not set", KSMConfigEnv)
		}
		return RunFromConfig(configBase64, os.Getenv(KSMRecordUIDEnv))
	case providerGCP:
		ka, gcp, err := loadScimParametersFromGCP(ctx)
		if err != nil {
			return nil, err
		}
		return runSync(ka, gcp)
	default:
		return nil, fmt.Errorf("unknown %s value %q (expected %q or %q)", SecretProviderEnv, provider, providerKSM, providerGCP)
	}
}

func RunFromConfig(configBase64, recordUID string) (*scim.SyncStat, error) {
	config := ksm.NewMemoryKeyValueStorage(configBase64)
	sm := ksm.NewSecretsManager(&ksm.ClientOptions{
		Config: config,
	})

	var filter []string
	if len(recordUID) > 0 {
		filter = append(filter, recordUID)
	}

	records, err := sm.GetSecrets(filter)
	if err != nil {
		return nil, err
	}

	scimRecord, err := findScimRecord(records)
	if err != nil {
		return nil, err
	}

	ka, gcp, err := scim.LoadScimParametersFromRecord(scimRecord)
	if err != nil {
		return nil, err
	}

	return runSync(ka, gcp)
}

// runSync builds the Google endpoint and SCIM sync from resolved parameters and
// executes it. Shared by the KSM and GCP provider paths.
func runSync(ka *scim.ScimEndpointParameters, gcp *scim.GoogleEndpointParameters) (*scim.SyncStat, error) {
	googleEndpoint := scim.NewGoogleEndpoint(gcp.Credentials, gcp.AdminAccount, gcp.ScimGroups)
	sync := scim.NewScimSync(googleEndpoint, ka.Url, ka.Token)
	sync.SetVerbose(ka.Verbose)
	sync.SetDestructive(ka.Destructive)

	return sync.Sync()
}

func findScimRecord(records []*ksm.Record) (*ksm.Record, error) {
	for _, r := range records {
		if r.Type() != "login" {
			continue
		}
		webURL := r.GetFieldValueByType("url")
		if len(webURL) == 0 {
			continue
		}
		uri, err := url.Parse(webURL)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(uri.Path, "/api/rest/scim/v2/") {
			continue
		}
		if len(r.FindFiles("credentials.json")) == 0 {
			continue
		}
		return r, nil
	}
	return nil, errors.New("SCIM record was not found. Make sure the record is valid and shared to KSM application")
}

func PrintStatistics(w io.Writer, syncStat *scim.SyncStat) {
	if syncStat == nil {
		return
	}
	if len(syncStat.SuccessGroups) > 0 {
		_, _ = fmt.Fprintf(w, "Group Success:\n")
		for _, txt := range syncStat.SuccessGroups {
			_, _ = fmt.Fprintf(w, "\t%s\n", txt)
		}
	}
	if len(syncStat.FailedGroups) > 0 {
		_, _ = fmt.Fprintf(w, "Group Failure:\n")
		for _, txt := range syncStat.FailedGroups {
			_, _ = fmt.Fprintf(w, "\t%s\n", txt)
		}
	}
	if len(syncStat.SuccessUsers) > 0 {
		_, _ = fmt.Fprintf(w, "User Success:\n")
		for _, txt := range syncStat.SuccessUsers {
			_, _ = fmt.Fprintf(w, "\t%s\n", txt)
		}
	}
	if len(syncStat.FailedUsers) > 0 {
		_, _ = fmt.Fprintf(w, "User Failure:\n")
		for _, txt := range syncStat.FailedUsers {
			_, _ = fmt.Fprintf(w, "\t%s\n", txt)
		}
	}
	if len(syncStat.SuccessMembership) > 0 {
		_, _ = fmt.Fprintf(w, "Membership Success:\n")
		for _, txt := range syncStat.SuccessMembership {
			_, _ = fmt.Fprintf(w, "\t%s\n", txt)
		}
	}
	if len(syncStat.FailedMembership) > 0 {
		_, _ = fmt.Fprintf(w, "Membership Failure:\n")
		for _, txt := range syncStat.FailedMembership {
			_, _ = fmt.Fprintf(w, "\t%s\n", txt)
		}
	}
}
