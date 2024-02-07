package main

import (
	"errors"
	"fmt"
	ksm "github.com/keeper-security/secrets-manager-go/core"
	"keepersecurity.com/ksm-scim/scim"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
)

func main() {
	var err error
	var filePath = "config.base64"
	if _, err = os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		var homeDir string
		if homeDir, err = os.UserHomeDir(); err != nil {
			log.Fatal(err)
		}
		filePath = path.Join(homeDir, filePath)
	}
	var data []byte
	if data, err = os.ReadFile(filePath); err != nil {
		log.Fatal(err)
	}
	var config = ksm.NewMemoryKeyValueStorage(string(data))
	var sm = ksm.NewSecretsManager(&ksm.ClientOptions{
		Config: config,
	})
	var filter []string
	if len(os.Args) == 2 {
		filter = append(filter, os.Args[1])
	}

	var records []*ksm.Record
	if records, err = sm.GetSecrets(filter); err != nil {
		log.Fatal(err)
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
		if uri, err = url.Parse(webUrl); err != nil {
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
		log.Fatal("SCIM record was not found. Make sure the record is valid and shared to KSM application")
	}
	var files = scimRecord.FindFiles("credentials.json")
	var credentials = files[0].GetFileData()
	var subject = scimRecord.GetFieldValueByType("login")
	var scimGroup = scimRecord.GetCustomFieldValueByLabel("SCIM Group")
	var googleEndpoint = scim.NewGoogleEndpoint(credentials, subject, scimGroup)

	var scimUrl = scimRecord.GetFieldValueByType("url")
	var token = scimRecord.Password()

	var sync = scim.NewScimSync(googleEndpoint, scimUrl, token)
	var syncStat *scim.SyncStat
	if syncStat, err = sync.Sync(); err != nil {
		log.Fatal(err.Error())
	}
	if len(syncStat.SuccessGroups) > 0 {
		fmt.Printf("Group Success:\n")
		for _, txt := range syncStat.SuccessGroups {
			fmt.Printf("\t%s\n", txt)
		}
	}
	if len(syncStat.FailedGroups) > 0 {
		fmt.Printf("Group Failure:\n")
		for _, txt := range syncStat.FailedGroups {
			fmt.Printf("\t%s\n", txt)
		}
	}
	if len(syncStat.SuccessUsers) > 0 {
		fmt.Printf("User Success:\n")
		for _, txt := range syncStat.SuccessUsers {
			fmt.Printf("\t%s\n", txt)
		}
	}
	if len(syncStat.FailedUsers) > 0 {
		fmt.Printf("User Failure:\n")
		for _, txt := range syncStat.FailedUsers {
			fmt.Printf("\t%s\n", txt)
		}
	}
	if len(syncStat.SuccessMembership) > 0 {
		fmt.Printf("Membership Success:\n")
		for _, txt := range syncStat.SuccessMembership {
			fmt.Printf("\t%s\n", txt)
		}
	}
	if len(syncStat.FailedMembership) > 0 {
		fmt.Printf("Membership Failure:\n")
		for _, txt := range syncStat.FailedMembership {
			fmt.Printf("\t%s\n", txt)
		}
	}
}
