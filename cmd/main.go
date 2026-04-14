package main

import (
	"errors"
	"fmt"
	ksm "github.com/keeper-security/secrets-manager-go/core"
	"keepersecurity.com/ksm-scim/scim"
	"log"
	"os"
	"path"
)

func main() {
	var err error
	var provider scim.SecretProvider

	var filePath = "config.base64"
	if _, err = os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		var homeDir string
		if homeDir, err = os.UserHomeDir(); err == nil {
			filePath = path.Join(homeDir, filePath)
		}
	}

	if _, err = os.Stat(filePath); err == nil {
		// KSM Provider Flow
		var data []byte
		if data, err = os.ReadFile(filePath); err != nil {
			log.Fatal(err)
		}
		var config = ksm.NewMemoryKeyValueStorage(string(data))
		var filter []string
		if len(os.Args) == 2 {
			filter = append(filter, os.Args[1])
		}
		provider = scim.NewKsmProvider(config, filter)
	} else if gcpSecretName := os.Getenv("GCP_SECRET_NAME"); gcpSecretName != "" {
		// GCP Provider Flow
		provider = scim.NewGcpProvider(gcpSecretName)
	} else {
		log.Fatal("Missing configuration: Could not find config.base64 locally or in home directory, and GCP_SECRET_NAME is not set.")
	}

	var ka *scim.ScimEndpointParameters
	var gcp *scim.GoogleEndpointParameters
	if ka, gcp, err = provider.Load(); err != nil {
		log.Fatal(err)
	}

	var googleEndpoint = scim.NewGoogleEndpoint(gcp.Credentials, gcp.AdminAccount, gcp.ScimGroups)

	var sync = scim.NewScimSync(googleEndpoint, ka.Url, ka.Token)
	sync.SetVerbose(ka.Verbose)
	sync.SetDestructive(ka.Destructive)

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
