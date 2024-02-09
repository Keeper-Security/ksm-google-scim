package scim

import (
	"errors"
	"fmt"
	"golang.org/x/text/cases"
)

// NewScimSync creates IScimSync interface for syncing with external CRMs
// source: external CRM data source
// url: base SCIM URL
// token: SCIM token
func NewScimSync(source ICrmDataSource, url string, token string) IScimSync {
	return &sync{
		source:  source,
		baseUrl: url,
		token:   token,
	}
}

type sync struct {
	source     ICrmDataSource
	scimUsers  map[string]*scimUser
	scimGroups map[string]*scimGroup
	baseUrl    string
	token      string
}

func (s *sync) Source() ICrmDataSource {
	return s.source
}
func (s *sync) Sync() (stat *SyncStat, err error) {
	if err = s.Source().Populate(); err != nil {
		return
	}
	if err = s.populateScim(); err != nil {
		return
	}
	var syncStat = new(SyncStat)
	if syncStat.SuccessGroups, syncStat.FailedGroups, err = s.syncGroups(); err != nil {
		return
	}
	if syncStat.SuccessUsers, syncStat.FailedUsers, err = s.syncUsers(); err != nil {
		return
	}
	if syncStat.SuccessMembership, syncStat.FailedMembership, err = s.syncMembership(); err != nil {
		return
	}
	stat = syncStat
	return
}

func (s *sync) syncGroups() (successes []string, failures []string, err error) {
	if s.scimGroups == nil {
		err = errors.New("SCIM groups were not populated")
		return
	}
	var keeperGroups = make(map[string]*scimGroup)
	for k, v := range s.scimGroups {
		keeperGroups[k] = v
	}

	var externalGroups = make(map[string]*Group)
	s.source.Groups(func(group *Group) {
		externalGroups[group.Id] = group
	})

	var er1 error
	var fold = cases.Fold()

	for matchRound := 0; matchRound < 3; matchRound++ {
		if len(keeperGroups) == 0 || len(externalGroups) == 0 {
			break
		}

		var groupLookup = make(map[string]*scimGroup)
		switch matchRound {
		case 0:
			for _, v := range keeperGroups {
				groupLookup[v.ExternalId] = v
			}
		case 1:
			for _, v := range keeperGroups {
				groupLookup[fold.String(v.Name)] = v
			}
		case 2:
			var extKeys []string
			for k := range externalGroups {
				extKeys = append(extKeys, k)
			}
			var scimKeys []string
			for k, v := range keeperGroups {
				if len(v.ExternalId) > 0 {
					scimKeys = append(scimKeys, k)
				}
			}
			var minKeys = len(extKeys)
			if minKeys > len(scimKeys) {
				minKeys = len(scimKeys)
			}
			for i := 0; i < minKeys; i++ {
				groupLookup[extKeys[i]] = keeperGroups[scimKeys[i]]
			}
		}

		for _, group := range externalGroups {
			var key string
			switch matchRound {
			case 0, 2:
				key = group.Id
			case 1:
				key = fold.String(group.Name)
			default:
				continue
			}

			if keeperGroup, ok := groupLookup[key]; ok {
				var value = make(map[string]any)
				if keeperGroup.ExternalId != group.Id {
					value["externalId"] = group.Id
				}
				if keeperGroup.Name != group.Name {
					value["displayName"] = group.Name
				}

				if len(value) > 0 {
					var op = make(map[string]any)
					op["op"] = "replace"
					op["value"] = value
					var payload = make(map[string]any)
					payload["schemas"] = []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"}
					payload["Operations"] = []any{op}
					if er1 = s.patchResource("Groups", keeperGroup.Id, payload); er1 == nil {
						keeperGroup.ExternalId = group.Id
						keeperGroup.Name = group.Name
						successes = append(successes, fmt.Sprintf("SCIM updated group \"%s\"", group.Name))
					} else {
						failures = append(failures, fmt.Sprintf("PATCH group \"%s\" error: %s", group.Name, er1.Error()))
					}
				}
				delete(keeperGroups, keeperGroup.Id)
				delete(externalGroups, group.Id)
			}
		}
	}
	if len(externalGroups) > 0 {
		for _, group := range externalGroups {
			var payload = make(map[string]any)
			payload["schemas"] = []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}
			payload["displayName"] = group.Name
			payload["externalId"] = group.Id

			var added map[string]any
			if added, er1 = s.postResource("Groups", payload); er1 == nil {
				if sg := parseScimGroup(added); sg != nil {
					s.scimGroups[sg.Id] = sg
				}
				successes = append(successes, fmt.Sprintf("SCIM added group \"%s\"", group.Name))
			} else {
				failures = append(failures, fmt.Sprintf("POST group \"%s\" error: %s", group.Name, er1.Error()))
			}
		}
	}

	if len(keeperGroups) > 0 {
		for groupId, group := range keeperGroups {
			if len(group.ExternalId) > 0 {
				if er1 = s.deleteResource("Groups", groupId); er1 == nil {
					delete(s.scimGroups, groupId)
					successes = append(successes, fmt.Sprintf("SCIM deleted group \"%s\"", group.Name))
				} else {
					failures = append(failures, fmt.Sprintf("DELETE group \"%s\" error: %s", group.Name, er1))
				}
			} else {
				failures = append(failures, fmt.Sprintf("DELETE group \"%s\": delete skipped since the group is not controlled by SCIM", group.Name))
			}
		}
	}
	return
}

func (s *sync) syncUsers() (successes []string, failures []string, err error) {
	if s.scimUsers == nil {
		err = errors.New("SCIM users were not populated")
		return
	}
	var keeperUsers = make(map[string]*scimUser)
	for k, v := range s.scimUsers {
		keeperUsers[k] = v
	}

	var externalUsers = make(map[string]*User)
	s.source.Users(func(user *User) {
		externalUsers[user.Id] = user
	})

	var er1 error
	var fold = cases.Fold()
	var ok bool

	if len(keeperUsers) > 0 && len(externalUsers) > 0 {
		var userLookup = make(map[string]*scimUser)
		for _, v := range s.scimUsers {
			userLookup[fold.String(v.Email)] = v
		}

		for _, user := range externalUsers {
			var keeperUser *scimUser
			if keeperUser, ok = userLookup[fold.String(user.Email)]; !ok {
				continue
			}
			var value = make(map[string]any)
			if keeperUser.ExternalId != user.Id {
				value["externalId"] = user.Id
			}
			if keeperUser.FullName != user.FullName {
				value["displayName"] = user.FullName
			}
			if keeperUser.LastName != user.LastName {
				value["name.familyName"] = user.LastName
			}
			if keeperUser.FirstName != user.FirstName {
				value["name.givenName"] = user.FirstName
			}
			if keeperUser.Active != user.Active {
				value["active"] = user.Active
			}
			if len(value) > 0 {
				var op = make(map[string]any)
				op["op"] = "replace"
				op["value"] = value
				var payload = make(map[string]any)
				payload["schemas"] = []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"}
				payload["Operations"] = []any{op}
				if er1 = s.patchResource("Users", keeperUser.Id, payload); er1 == nil {
					keeperUser.ExternalId = user.Id
					keeperUser.FullName = user.FullName
					keeperUser.FirstName = user.FirstName
					keeperUser.LastName = user.LastName
					keeperUser.Active = user.Active
					successes = append(successes, fmt.Sprintf("SCIM updated user \"%s\"", user.Email))
				} else {
					failures = append(failures, fmt.Sprintf("PATCH user \"%s\" error: %s", user.Email, er1.Error()))
				}
			}
			delete(externalUsers, user.Id)
			delete(keeperUsers, keeperUser.Id)
		}
	}

	if len(externalUsers) > 0 {
		for _, user := range externalUsers {
			if !user.Active {
				continue
			}
			var payload = make(map[string]any)
			payload["schemas"] = []string{"urn:ietf:params:scim:schemas:core:2.0:User",
				"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"}
			payload["userName"] = user.Email
			payload["externalId"] = user.Id
			payload["displayName"] = user.FullName
			var name = make(map[string]any)
			name["givenName"] = user.FirstName
			name["familyName"] = user.LastName
			payload["name"] = name
			payload["active"] = user.Active
			if payload, er1 = s.postResource("Users", payload); er1 == nil {
				if au := parseScimUser(payload); au != nil {
					s.scimUsers[au.Id] = au
				}
				successes = append(successes, fmt.Sprintf("SCIM added user \"%s\"", user.Email))
			} else {
				failures = append(failures, fmt.Sprintf("POST user \"%s\" error: %s", user.Email, er1.Error()))
			}
		}
	}
	if len(keeperUsers) > 0 {
		for _, user := range keeperUsers {
			if !user.Active {
				continue
			}
			if er1 = s.deleteResource("Users", user.Id); er1 == nil {
				delete(s.scimUsers, user.Id)
				successes = append(successes, fmt.Sprintf("SCIM deleted user \"%s\"", user.Email))
			} else {
				failures = append(failures, fmt.Sprintf("DELETE user \"%s\" error: %s", user.Email, er1.Error()))
			}
		}
	}
	return
}

func (s *sync) syncMembership() (successes []string, failures []string, err error) {
	var fold = cases.Fold()
	var keeperUserLookup = make(map[string]*scimUser)
	for _, v := range s.scimUsers {
		keeperUserLookup[fold.String(v.Email)] = v
	}
	var keeperGroupMap = make(map[string]string)
	for _, v := range s.scimGroups {
		keeperGroupMap[v.ExternalId] = v.Id
	}
	var ok bool
	var keeperUser *scimUser
	s.source.Users(func(user *User) {
		if keeperUser, ok = keeperUserLookup[fold.String(user.Email)]; !ok {
			return
		}
		var keeperUserGroups = MakeSet[string](keeperUser.Groups)
		var skipDeletion = false
		var addGroups, removeGroups []string
		for _, externalGroupId := range user.Groups {
			var keeperGroupId string
			if keeperGroupId, ok = keeperGroupMap[externalGroupId]; ok {
				if keeperUserGroups.Has(keeperGroupId) {
					keeperUserGroups.Delete(keeperGroupId)
				} else {
					addGroups = append(addGroups, keeperGroupId)
				}
			} else {
				// TODO ?????
				skipDeletion = true
			}
		}
		if !skipDeletion && len(keeperUserGroups) > 0 {
			removeGroups = append(removeGroups, keeperUserGroups.ToArray()...)
		}
		if len(addGroups) > 0 || len(removeGroups) > 0 {
			var operations []any
			var values []any
			for _, groupId := range addGroups {
				var value = make(map[string]any)
				value["value"] = groupId
				values = append(values, value)
			}
			if len(values) > 0 {
				var op = make(map[string]any)
				op["op"] = "add"
				op["path"] = "groups"
				op["value"] = values
				operations = append(operations, op)
			}
			values = nil
			for _, groupId := range removeGroups {
				var value = make(map[string]any)
				value["value"] = groupId
				values = append(values, value)
			}
			if len(values) > 0 {
				var op = make(map[string]any)
				op["op"] = "remove"
				op["path"] = "groups"
				op["value"] = values
				operations = append(operations, op)
			}

			var payload = make(map[string]any)
			payload["schemas"] = []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"}
			payload["Operations"] = operations

			if er1 := s.patchResource("Users", keeperUser.Id, payload); er1 == nil {
				successes = append(successes, fmt.Sprintf("SCIM changed user \"%s\" membership: %d added; %d removed", keeperUser.Email, len(addGroups), len(removeGroups)))
			} else {
				failures = append(failures, fmt.Sprintf("PATCH user \"%s\" membership error: %s", keeperUser.Email, er1.Error()))
			}
		}
	})

	return
}
