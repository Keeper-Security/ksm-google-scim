package ksm_google_scim

import (
	"context"
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	ksm "github.com/keeper-security/secrets-manager-go/core"
	"io"
	"keepersecurity.com/ksm-scim/scim"
	"log"
	"net/http"
	"os"
)

func init() {
	// Register an HTTP function with the Functions Framework
	functions.HTTP("GcpScimSyncHttp", gcpScimSyncHttp)
	functions.CloudEvent("GcpScimSyncPubSub", gcpScimSyncPubSub)
}

const ksmConfigName = "KSM_CONFIG_BASE64"
const ksmRecordUid = "KSM_RECORD_UID"

func runScimSync() (syncStat *scim.SyncStat, err error) {
	var provider scim.SecretProvider

	var configBase64 = os.Getenv(ksmConfigName)
	var gcpSecretName = os.Getenv("GCP_SECRET_NAME")

	if len(configBase64) > 0 {
		var config = ksm.NewMemoryKeyValueStorage(configBase64)
		var filter []string
		var recordUid = os.Getenv(ksmRecordUid)
		if len(recordUid) > 0 {
			filter = append(filter, recordUid)
		}
		provider = scim.NewKsmProvider(config, filter)
	} else if len(gcpSecretName) > 0 {
		provider = scim.NewGcpProvider(gcpSecretName)
	} else {
		err = errors.New(fmt.Sprintf("Missing configuration: Set either %s or GCP_SECRET_NAME", ksmConfigName))
		log.Println(err)
		return
	}

	var ka *scim.ScimEndpointParameters
	var gcp *scim.GoogleEndpointParameters
	if ka, gcp, err = provider.Load(); err != nil {
		log.Println(err)
		return
	}
	var googleEndpoint = scim.NewGoogleEndpoint(gcp.Credentials, gcp.AdminAccount, gcp.ScimGroups)
	var sync = scim.NewScimSync(googleEndpoint, ka.Url, ka.Token)
	sync.SetVerbose(ka.Verbose)
	sync.SetDestructive(ka.Destructive)

	if syncStat, err = sync.Sync(); err == nil {
		printStatistics(os.Stdout, syncStat)
	}

	return
}

func printStatistics(w io.Writer, syncStat *scim.SyncStat) {
	if syncStat != nil {
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
}

// Function gcpScimSync is an HTTP handler
func gcpScimSyncHttp(w http.ResponseWriter, r *http.Request) {
	var syncStat, err = runScimSync()
	if err == nil {
		printStatistics(w, syncStat)
	} else {
		log.Printf("Sync failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// helloPubSub consumes a CloudEvent message and extracts the Pub/Sub message.
func gcpScimSyncPubSub(_ context.Context, _ event.Event) (err error) {
	_, err = runScimSync()
	return
}
