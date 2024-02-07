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
	"net/url"
	"os"
	"strings"
)

func init() {
	// Register an HTTP function with the Functions Framework
	functions.HTTP("GcpScimSyncHttp", gcpScimSyncHttp)
	functions.CloudEvent("GcpScimSyncPubSub", gcpScimSyncPubSub)
}

const ksmConfigName = "KSM_CONFIG_BASE64"
const ksmRecordUid = "KSM_RECORD_UID"

func runScimSync() (syncStat *scim.SyncStat, err error) {
	var configBase64 = os.Getenv(ksmConfigName)
	if len(configBase64) == 0 {
		err = errors.New(fmt.Sprintf("Environment variable \"%s\" is not set", ksmConfigName))
		log.Println(err)
		return
	}

	var config = ksm.NewMemoryKeyValueStorage(configBase64)
	var sm = ksm.NewSecretsManager(&ksm.ClientOptions{
		Config: config,
	})

	var filter []string
	var recordUid = os.Getenv(ksmRecordUid)
	if len(recordUid) > 0 {
		filter = append(filter, recordUid)
	}

	var records []*ksm.Record
	if records, err = sm.GetSecrets(filter); err != nil {
		log.Println(err)
		return
	}

	var scimRecord *ksm.Record
	for _, r := range records {
		if r.Type() != "login" {
			continue
		}
		var webUrl = r.GetFieldValueByType("url")
		if len(webUrl) == 0 {
			continue
		}
		var uri *url.URL
		var er1 error
		if uri, er1 = url.Parse(webUrl); er1 != nil {
			continue
		}
		if !strings.HasPrefix(uri.Path, "/api/rest/scim/v2/") {
			continue
		}

		var files = r.FindFiles("credentials.json")
		if len(files) == 0 {
			continue
		}
		scimRecord = r
		break
	}
	if scimRecord == nil {
		err = errors.New("SCIM record was not found. Make sure the record is valid and shared to KSM application")
		log.Println(err)
		return
	}

	var files = scimRecord.FindFiles("credentials.json")
	var credentials = files[0].GetFileData()
	var subject = scimRecord.GetFieldValueByType("login")

	var fields = scimRecord.GetCustomFieldsByLabel("SCIM Group")
	if len(fields) == 0 {
		err = errors.New("\"SCIM Group\" custom field was not found. Please add a custom field \"SCIM Group\" to your record")
		log.Println(err)
		return
	}
	var scimGroups = scim.ParseScimGroups(fields)
	if len(fields) == 0 {
		err = errors.New("\"SCIM Group\" custom field does not contain any value")
		log.Println(err)
		return
	}
	var googleEndpoint = scim.NewGoogleEndpoint(credentials, subject, scimGroups)

	var scimUrl = scimRecord.GetFieldValueByType("url")
	var token = scimRecord.Password()

	var sync = scim.NewScimSync(googleEndpoint, scimUrl, token)
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
		log.Fatal(err)
	}
}

// helloPubSub consumes a CloudEvent message and extracts the Pub/Sub message.
func gcpScimSyncPubSub(_ context.Context, _ event.Event) (err error) {
	_, err = runScimSync()
	return
}
