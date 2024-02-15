package scim

import (
	"errors"
	ksm "github.com/keeper-security/secrets-manager-go/core"
	"strconv"
)

func LoadScimParametersFromRecord(scimRecord *ksm.Record) (ka *ScimEndpointParameters, gcp *GoogleEndpointParameters, err error) {
	var files = scimRecord.FindFiles("credentials.json")
	var credentials = files[0].GetFileData()
	var subject = scimRecord.GetFieldValueByType("login")

	var scimGroups []string

	var fields = scimRecord.GetCustomFieldsByLabel("SCIM Group")
	if len(fields) != 0 {
		scimGroups = append(scimGroups, ParseScimGroups(fields)...)
	}
	fields = scimRecord.GetCustomFieldsByLabel("SCIM Groups")
	if len(fields) != 0 {
		scimGroups = append(scimGroups, ParseScimGroups(fields)...)
	}

	if len(scimGroups) == 0 {
		err = errors.New("\"SCIM Group\" custom field is missing or does not contain any value")
		return
	}

	gcp = &GoogleEndpointParameters{
		AdminAccount: subject,
		Credentials:  credentials,
		ScimGroups:   scimGroups,
	}

	ka = &ScimEndpointParameters{
		Url:   scimRecord.GetFieldValueByType("url"),
		Token: scimRecord.Password(),
	}

	var ok bool
	var bv bool
	fields = scimRecord.GetCustomFieldsByLabel("Verbose")
	if len(fields) > 0 {
		if bv, ok = toBoolean(fields[0]["value"]); ok {
			ka.Verbose = bv
		}
	}

	var sv string
	fields = scimRecord.GetCustomFieldsByLabel("Destructive")
	if len(fields) > 0 {
		var value = fields[0]["value"]
		var av []any
		if av, ok = value.([]any); ok {
			if len(av) > 0 && av[0] != nil {
				if sv, ok = av[0].(string); ok {
					if iv, er1 := strconv.Atoi(sv); er1 == nil {
						ka.Destructive = int32(iv)
					} else {
						ka.Destructive = -1
					}
				}
			}
		}
	}
	return
}
