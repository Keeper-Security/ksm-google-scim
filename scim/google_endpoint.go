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
	scimGroup      string
}

// NewGoogleEndpoint creates an ICrmDataSource for accessing Users and Groups in Google Workspace
// credentials: GCP service account JWT credentials
// subject: Google Workspace admin account
// scimGroup: Google Workspace Group that
func NewGoogleEndpoint(credentials []byte, subject string, scimGroup string) ICrmDataSource {
	return &googleEndpoint{
		jwtCredentials: credentials,
		subject:        subject,
		scimGroup:      scimGroup,
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

	ge.users = make(map[string]*User)
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
				ge.users[gu.Id] = gu
			}
		}
	} else {
		err = errors.New("google directory API: error querying users")
		return
	}

	ge.groups = make(map[string]*Group)
	var gl = directory.Groups.List().Customer("my_customer")
	if gl != nil {
		var groups *admin.Groups
		if groups, err = gl.Do(); err == nil {
			for _, g := range groups.Groups {
				var gg = &Group{
					Id:   g.Id,
					Name: g.Name,
				}
				ge.groups[gg.Id] = gg
			}
		}
	} else {
		err = errors.New("google directory API: error querying users")
		return
	}

	var scimGroupId string
	if len(ge.scimGroup) > 0 {
		for _, v := range ge.groups {
			if strings.EqualFold(v.Name, ge.scimGroup) {
				scimGroupId = v.Id
				break
			}
		}
	}
	if len(scimGroupId) > 0 {
		var members *admin.Members
		if members, err = directory.Members.List(scimGroupId).Do(); err != nil {
			return
		}
		var scimUsers = NewSet[string]()
		for _, m := range members.Members {
			scimUsers.Add(m.Id)
		}

		var allUsers = NewSet[string]()
		for k := range ge.users {
			allUsers.Add(k)
		}

		allUsers.Difference(scimUsers.ToArray())
		for _, k := range allUsers.ToArray() {
			delete(ge.users, k)
		}
		delete(ge.groups, scimGroupId)
	}

	var allGroups = NewSet[string]()
	for k := range ge.groups {
		allGroups.Add(k)
	}
	for _, groupId := range allGroups.ToArray() {
		var members *admin.Members
		if members, err = directory.Members.List(groupId).Do(); err != nil {
			return
		}
		var used = false
		for _, m := range members.Members {
			if u, ok := ge.users[m.Id]; ok {
				used = true
				u.Groups = append(u.Groups, groupId)
			}
		}
		if !used {
			delete(ge.groups, groupId)
		}
	}

	return
}
