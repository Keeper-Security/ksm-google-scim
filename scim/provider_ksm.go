package scim

import (
	"errors"
	"net/url"
	"strings"

	ksm "github.com/keeper-security/secrets-manager-go/core"
)

type KsmProvider struct {
	Config ksm.IKeyValueStorage
	Filter []string
}

func NewKsmProvider(config ksm.IKeyValueStorage, filter []string) SecretProvider {
	return &KsmProvider{
		Config: config,
		Filter: filter,
	}
}

func (p *KsmProvider) Load() (*ScimEndpointParameters, *GoogleEndpointParameters, error) {
	sm := ksm.NewSecretsManager(&ksm.ClientOptions{
		Config: p.Config,
	})

	records, err := sm.GetSecrets(p.Filter)
	if err != nil {
		return nil, nil, err
	}

	var scimRecord *ksm.Record
	for _, r := range records {
		if r.Type() != "login" {
			continue
		}
		webUrl := r.GetFieldValueByType("url")
		if len(webUrl) == 0 {
			continue
		}
		uri, err := url.Parse(webUrl)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(uri.Path, "/api/rest/scim/v2/") {
			continue
		}
		files := r.FindFiles("credentials.json")
		if len(files) == 0 {
			continue
		}
		scimRecord = r
		break
	}

	if scimRecord == nil {
		return nil, nil, errors.New("SCIM record was not found. Make sure the record is valid and shared to KSM application")
	}

	return LoadScimParametersFromRecord(scimRecord)
}
