package scim

import (
	"errors"
	ksm "github.com/keeper-security/secrets-manager-go/core"
)

func LoadScimParametersFromRecord(scimRecord *ksm.Record) (ka *ScimEndpointParameters, gcp *GoogleEndpointParameters, err error) {
	var files = scimRecord.FindFiles("credentials.json")
	var credentials = files[0].GetFileData()
	var subject = scimRecord.GetFieldValueByType("login")

	var fields = scimRecord.GetCustomFieldsByLabel("SCIM Group")
	if len(fields) == 0 {
		err = errors.New("\"SCIM Group\" custom field was not found. Please add a custom field \"SCIM Group\" to your record")
		return
	}
	var scimGroups = ParseScimGroups(fields)
	if len(scimGroups) == 0 {
		err = errors.New("\"SCIM Group\" custom field does not contain any value")
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

	fields = scimRecord.GetCustomFieldsByLabel("Destructive")
	if len(fields) > 0 {
		if bv, ok = toBoolean(fields[0]["value"]); ok {
			ka.Destructive = bv
		}
	}
	return
}
