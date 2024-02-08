package scim

import (
	"context"
	"errors"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
	"strings"
)

type googleEndpoint struct {
	users          map[string]*User
	groups         map[string]*Group
	jwtCredentials []byte
	subject        string
	scimGroups     []string
}

// NewGoogleEndpoint creates an ICrmDataSource for accessing Users and Groups in Google Workspace
// credentials: GCP service account JWT credentials
// subject: Google Workspace admin account
// scimGroup: Google Workspace Group that
func NewGoogleEndpoint(credentials []byte, subject string, scimGroups []string) ICrmDataSource {
	return &googleEndpoint{
		jwtCredentials: credentials,
		subject:        subject,
		scimGroups:     scimGroups,
	}
}

func (ge *googleEndpoint) Users(cb func(*User)) {
	if ge.users != nil {
		for _, v := range ge.users {
			cb(v)
		}
	}
}

func (ge *googleEndpoint) Groups(cb func(*Group)) {
	if ge.users != nil {
		for _, v := range ge.groups {
			cb(v)
		}
	}
}

func (ge *googleEndpoint) Populate() (err error) {
	params := google.CredentialsParams{
		Scopes: []string{admin.AdminDirectoryUserReadonlyScope,
			admin.AdminDirectoryGroupReadonlyScope, admin.AdminDirectoryGroupMemberReadonlyScope},
		Subject: ge.subject,
	}
	var ctx = context.Background()
	cred, _ := google.CredentialsFromJSONWithParams(ctx, ge.jwtCredentials, params)
	var directory *admin.Service
	if directory, err = admin.NewService(ctx, option.WithCredentials(cred)); err != nil {
		return
	}

	var scimGroups = NewSet[string]()
	for _, x := range ge.scimGroups {
		x = strings.TrimSpace(x)
		if len(x) == 0 {
			continue
		}
		for _, y := range strings.Split(x, "\n") {
			y = strings.TrimSpace(y)
			if len(y) == 0 {
				continue
			}
			for _, z := range strings.Split(y, ",") {
				z = strings.TrimSpace(z)
				if len(z) == 0 {
					continue
				}
				scimGroups.Add(strings.ToLower(z))
			}
		}
	}
	if len(scimGroups) == 0 {
		err = errors.New("could not resolve \"SCIM Group\" content to groups")
		return
	}

	ge.users = make(map[string]*User)
	var userLookup = make(map[string]*User)
	var ul = directory.Users.List().Customer("my_customer")
	if ul != nil {
		var users *admin.Users
		if users, err = ul.Do(); err == nil {
			for _, u := range users.Users {
				var gu = &User{
					Id:     u.Id,
					Email:  u.PrimaryEmail,
					Active: !u.Suspended,
				}
				if u.Name != nil {
					gu.FirstName = u.Name.GivenName
					gu.LastName = u.Name.FamilyName
					if len(u.Name.FullName) > 0 {
						gu.FullName = u.Name.FullName
					} else {
						gu.FullName = strings.TrimSpace(strings.Join([]string{u.Name.GivenName, u.Name.FamilyName}, " "))
					}
				}
				userLookup[gu.Id] = gu
				if scimGroups.Has(strings.ToLower(gu.Email)) {
					ge.users[gu.Id] = gu
				}
			}
		}
	} else {
		err = errors.New("google directory API: error querying users")
		return
	}

	ge.groups = make(map[string]*Group)
	var groupLookup = make(map[string]*Group)
	var gl = directory.Groups.List().Customer("my_customer")
	if gl != nil {
		var groups *admin.Groups
		if groups, err = gl.Do(); err == nil {
			for _, g := range groups.Groups {
				var gg = &Group{
					Id:   g.Id,
					Name: g.Name,
				}
				groupLookup[gg.Id] = gg
				if scimGroups.Has(strings.ToLower(g.Email)) || scimGroups.Has(strings.ToLower(g.Name)) {
					ge.groups[gg.Id] = gg
				}
			}
		}
	} else {
		err = errors.New("google directory API: error querying users")
		return
	}

	if len(ge.groups) == 0 && len(ge.users) == 0 {
		err = errors.New("no Google Workspace groups could be resolved")
		return
	}

	var ok bool
	// expand embedded groups
	var membershipCache = make(map[string][]string)
	for groupId := range ge.groups {
		var groupIds = []string{groupId}
		var queuedIds = MakeSet[string](groupIds)
		var pos = 0
		for pos < len(groupIds) {
			var gId = groupIds[pos]
			pos++

			var memberIds []string
			if memberIds, ok = membershipCache[gId]; !ok {
				var members *admin.Members
				if members, err = directory.Members.List(gId).Do(); err != nil {
					return
				}
				for _, m := range members.Members {
					memberIds = append(memberIds, m.Id)
				}
				membershipCache[gId] = memberIds
			}
			var u *User
			var g *Group
			for _, mId := range memberIds {
				if u, ok = userLookup[mId]; ok {
					u.Groups = append(u.Groups, groupId)
					if _, ok = ge.users[u.Id]; !ok {
						ge.users[u.Id] = u
					}
				} else if g, ok = groupLookup[mId]; ok {
					if !queuedIds.Has(g.Id) {
						groupIds = append(groupIds, g.Id)
						queuedIds.Add(g.Id)
					}
				}
			}
		}
	}

	return
}
