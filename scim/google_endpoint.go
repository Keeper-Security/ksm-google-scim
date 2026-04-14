package scim

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
	"net/mail"
	"strings"
)

type googleEndpoint struct {
	users          map[string]*User
	groups         map[string]*Group
	jwtCredentials []byte
	subject        string
	scimGroups     []string
	logger         SyncDebugLogger
	loadErrors     bool
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
func (ge *googleEndpoint) DebugLogger() SyncDebugLogger {
	if ge.logger != nil {
		return ge.logger
	}
	return NilLogger
}
func (ge *googleEndpoint) SetDebugLogger(logger SyncDebugLogger) {
	ge.logger = logger
	if logger == nil {
		ge.logger = NilLogger
	} else {
	}
}
func (ge *googleEndpoint) LoadErrors() bool {
	return ge.loadErrors
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

func parseGoogleUser(gu *admin.User) (su *User) {
	su = &User{
		Id:     gu.Id,
		Email:  gu.PrimaryEmail,
		Active: !gu.Suspended,
	}
	if gu.Name != nil {
		su.FirstName = gu.Name.GivenName
		su.LastName = gu.Name.FamilyName
		if len(gu.Name.FullName) > 0 {
			su.FullName = gu.Name.FullName
		} else {
			su.FullName = strings.TrimSpace(strings.Join([]string{gu.Name.GivenName, gu.Name.FamilyName}, " "))
		}
	}
	return
}

func (ge *googleEndpoint) Populate() (err error) {
	ge.loadErrors = false
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
				scimGroups.Add(z)
			}
		}
	}
	if len(scimGroups) == 0 {
		err = errors.New("could not resolve \"SCIM Group\" content to groups")
		return
	}

	ge.users = make(map[string]*User)
	ge.groups = make(map[string]*Group)

	ge.DebugLogger()("Resolving \"SCIM Group\" content")
	var users *admin.Users
	var groups *admin.Groups
	for entry := range scimGroups {
		if strings.HasSuffix(entry, "*") && !strings.HasPrefix(entry, "*") {
			// Prefix name wildcard (e.g., "keeper-scim-*")
			var foundAny = false

			// Extract the prefix by removing the asterisk
			var prefix = strings.TrimSuffix(entry, "*")

			// 1. Try email prefix first (operator: ":")
			// Google Directory API syntax requires NO quotes and NO asterisk for prefix searches
			var emailQuery = fmt.Sprintf("email:%s*", prefix)
			ge.DebugLogger()(fmt.Sprintf("Sending Google API Query: %s", emailQuery))

			var el = directory.Groups.List().Customer("my_customer").Query(emailQuery)
			if egroups, err := el.Do(); err == nil && len(egroups.Groups) > 0 {
				foundAny = true
				for _, g := range egroups.Groups {
					ge.DebugLogger()(fmt.Sprintf("Found Google group \"%s\" matching email prefix \"%s\"", g.Name, entry))
					ge.groups[g.Id] = &Group{
						Id:   g.Id,
						Name: g.Name,
					}
				}
			} else if err != nil {
				ge.DebugLogger()(fmt.Sprintf("Google API error for email query '%s': %v", emailQuery, err))
			}

			// 2. Try name prefix (operator: ":")
			var nameQuery = fmt.Sprintf("name:%s*", prefix)
			ge.DebugLogger()(fmt.Sprintf("Sending Google API Query: %s", nameQuery))

			var nl = directory.Groups.List().Customer("my_customer").Query(nameQuery)
			if ngroups, err := nl.Do(); err == nil && len(ngroups.Groups) > 0 {
				foundAny = true
				for _, g := range ngroups.Groups {
					ge.DebugLogger()(fmt.Sprintf("Found Google group \"%s\" matching name prefix \"%s\"", g.Name, entry))
					ge.groups[g.Id] = &Group{
						Id:   g.Id,
						Name: g.Name,
					}
				}
			} else if err != nil {
				ge.DebugLogger()(fmt.Sprintf("Google API error for name query '%s': %v", nameQuery, err))
			}

			if !foundAny {
				ge.DebugLogger()(fmt.Sprintf("No groups found matching prefix wildcard \"%s\"", entry))
				ge.loadErrors = true
			}
		} else if strings.HasPrefix(entry, "*@") {
			// Domain email wildcard (e.g., "*@webfx.com")
			var domain = strings.TrimPrefix(entry, "*@")
			ge.DebugLogger()(fmt.Sprintf("Sending Google API Query (Domain filter): domain='%s'", domain))
			var gl = directory.Groups.List().Domain(domain)
			if groups, err := gl.Do(); err == nil && len(groups.Groups) > 0 {
				for _, g := range groups.Groups {
					ge.DebugLogger()(fmt.Sprintf("Found Google group \"%s\" matching domain \"%s\"", g.Name, domain))
					ge.groups[g.Id] = &Group{
						Id:   g.Id,
						Name: g.Name,
					}
				}
			} else {
				if err != nil {
					ge.DebugLogger()(fmt.Sprintf("Google API error for domain query '%s': %v", domain, err))
				}
				ge.DebugLogger()(fmt.Sprintf("No groups found matching domain wildcard \"%s\"", entry))
				ge.loadErrors = true
			}
		} else {
			// Exact match (Email or Name)
			var address *mail.Address
			if address, err = mail.ParseAddress(entry); err == nil {
				var gl = directory.Groups.List().Customer("my_customer").Query(fmt.Sprintf("email=%s", address.Address))
				if groups, err = gl.Do(); err == nil && len(groups.Groups) > 0 {
					for _, g := range groups.Groups {
						ge.DebugLogger()(fmt.Sprintf("Found Google group \"%s\" for email \"%s\"", g.Name, g.Email))
						ge.groups[g.Id] = &Group{
							Id:   g.Id,
							Name: g.Name,
						}
					}
				} else {
					var ul = directory.Users.List().Customer("my_customer").Query(fmt.Sprintf("email=%s", address.Address))
					if users, err = ul.Do(); err == nil && len(users.Users) > 0 {
						for _, u := range users.Users {
							ge.DebugLogger()(fmt.Sprintf("Found Google user for email \"%s\"", u.PrimaryEmail))
							var su = parseGoogleUser(u)
							ge.users[su.Id] = su
						}
					} else {
						ge.DebugLogger()(fmt.Sprintf("An email \"%s\" could not be resolved as either Google User or Group", address.Address))
						ge.loadErrors = true
					}
				}
			} else {
				var gl = directory.Groups.List().Customer("my_customer").Query(fmt.Sprintf("name='%s'", entry))
				if groups, err = gl.Do(); err == nil && len(groups.Groups) > 0 {
					for _, g := range groups.Groups {
						ge.DebugLogger()(fmt.Sprintf("Found Google group \"%s\" by name", g.Name))
						ge.groups[g.Id] = &Group{
							Id:   g.Id,
							Name: g.Name,
						}
					}
				} else {
					ge.DebugLogger()(fmt.Sprintf("A name \"%s\" could not be resolved to Google Group. Names are case sensitive", entry))
					ge.loadErrors = true
				}
			}
		}
	}

	if len(ge.groups) == 0 && len(ge.users) == 0 {
		err = errors.New("no Google Workspace groups could be resolved")
		return
	}

	ge.DebugLogger()("Loading all users")
	var userLookup = make(map[string]*User)
	if err = directory.Users.List().Customer("my_customer").MaxResults(200).Pages(ctx, func(users *admin.Users) error {
		var no = 0
		for _, u := range users.Users {
			var su = parseGoogleUser(u)
			userLookup[su.Id] = su
			no++
		}
		ge.DebugLogger()(fmt.Sprintf("User page contains %d element(s)", no))
		return nil
	}); err != nil {
		err = errors.New("google directory API: error querying users")
		return
	}
	ge.DebugLogger()(fmt.Sprintf("Total %d Google user(s) loaded", len(userLookup)))

	var ok bool
	// expand embedded groups
	var membershipCache = make(map[string][]string)
	for groupId, group := range ge.groups {
		var groupIds = []string{groupId}
		var queuedIds = MakeSet[string](groupIds)
		var pos = 0
		for pos < len(groupIds) {
			var gId = groupIds[pos]
			pos++

			var memberIds []string
			if memberIds, ok = membershipCache[gId]; !ok {
				if err = directory.Members.List(gId).Pages(ctx, func(members *admin.Members) error {
					for _, m := range members.Members {
						memberIds = append(memberIds, m.Id)
					}
					return nil
				}); err != nil {
					ge.DebugLogger()(fmt.Sprintf("Loaded group \"%s\" membership failed: %s", group.Name, err.Error()))
				}
				membershipCache[gId] = memberIds
			}
			for _, mId := range memberIds {
				var u *User
				if u, ok = userLookup[mId]; ok {
					u.Groups = append(u.Groups, groupId)
					if _, ok = ge.users[u.Id]; !ok {
						ge.users[u.Id] = u
					}
				} else {
					if !queuedIds.Has(mId) {
						groupIds = append(groupIds, mId)
						queuedIds.Add(mId)
					}
				}
			}
		}
	}

	return
}
