package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type scimUser struct {
	User
	ExternalId string
}

type scimGroup struct {
	Group
	ExternalId string
}

func parseScimGroup(groupObject map[string]any) (result *scimGroup) {
	var ok bool
	var id, name string
	if id, ok = toString(groupObject["id"]); ok {
		name, ok = toString(groupObject["displayName"])
	}
	if ok {
		result = new(scimGroup)
		result.Id = id
		result.Name = name
		result.ExternalId, _ = toString(groupObject["externalId"])
	}
	return
}

func parseScimUser(userObject map[string]any) (result *scimUser) {
	var ok bool
	var userId, email string
	if userId, ok = toString(userObject["id"]); ok {
		email, ok = toString(userObject["userName"])
	}
	if !ok {
		return
	}
	result = new(scimUser)
	result.Id = userId
	result.Email = email
	result.Active, _ = toBoolean(userObject["active"])
	result.ExternalId, _ = toString(userObject["externalId"])
	result.FullName, _ = toString(userObject["displayName"])
	var j any
	var jo map[string]any
	if j = userObject["name"]; j != nil {
		if jo, ok = j.(map[string]any); ok {
			result.FirstName, _ = toString(jo["givenName"])
			result.LastName, _ = toString(jo["familyName"])
		}
	}
	if j = userObject["groups"]; j != nil {
		var ja []any
		if ja, ok = j.([]any); ok {
			for _, j = range ja {
				if jo, ok = j.(map[string]any); ok {
					var groupId string
					if groupId, ok = toString(jo["value"]); ok {
						result.Groups = append(result.Groups, groupId)
					}
				}
			}
		}
	}
	return
}

func (s *sync) populateScim() (err error) {
	s.scimGroups = make(map[string]*scimGroup)
	if err = s.getResources("Groups", func(ro map[string]any) {
		if g := parseScimGroup(ro); g != nil {
			s.scimGroups[g.Id] = g
		}
	}); err != nil {
		return
	}

	s.scimUsers = make(map[string]*scimUser)
	if err = s.getResources("Users", func(ro map[string]any) {
		if user := parseScimUser(ro); user != nil {
			s.scimUsers[user.Id] = user
		}
	}); err != nil {
		return
	}
	return
}

func (s *sync) composeUrl(paths ...string) (result *url.URL, err error) {
	var uri *url.URL
	if uri, err = url.Parse(s.baseUrl); err != nil {
		return
	}
	var ruri *url.URL
	for _, path := range paths {
		if ruri, err = url.Parse(path); err != nil {
			return
		}
		if !strings.HasSuffix(uri.Path, "/") {
			uri.Path += "/"
		}
		uri = uri.ResolveReference(ruri)
	}

	result = uri
	return
}

func (s *sync) executeRequest(rq *http.Request) (response map[string]any, err error) {
	client := http.DefaultClient
	var rs *http.Response
	if rs, err = client.Do(rq); err != nil {
		return
	}
	var body []byte
	var contentType = rs.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/") {
		if body, err = io.ReadAll(rs.Body); err != nil {
			return
		}
	}
	if rs.StatusCode >= 300 {
		var scimUrl = rq.URL.String()
		if strings.HasPrefix(scimUrl, s.baseUrl) {
			scimUrl = scimUrl[len(s.baseUrl):]
			scimUrl = strings.Trim(scimUrl, "/")
		}
		if len(body) > 0 {
			err = fmt.Errorf("%s SCIM \"%s\" error: %s", rq.Method, scimUrl, string(body))
		} else {
			err = fmt.Errorf("%s SCIM \"%s\" error: Status code %d", rq.Method, scimUrl, rs.StatusCode)
		}
		return
	}
	if (rs.StatusCode == 200 || rs.StatusCode == 201) && len(body) > 0 {
		err = json.Unmarshal(body, &response)
	}
	return
}

func (s *sync) patchResource(resourceType string, resourceId string, payload any) (err error) {
	var uri *url.URL
	if uri, err = s.composeUrl(resourceType, resourceId); err != nil {
		return
	}

	var data []byte
	if data, err = json.Marshal(payload); err != nil {
		return
	}

	var rq *http.Request
	if rq, err = http.NewRequest("PATCH", uri.String(), bytes.NewBuffer(data)); err != nil {
		return
	}
	rq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.token))
	rq.Header.Add("Content-Type", "application/json")

	_, err = s.executeRequest(rq)
	return
}

func (s *sync) postResource(resourceType string, payload any) (resource map[string]any, err error) {
	var uri *url.URL
	if uri, err = s.composeUrl(resourceType); err != nil {
		return
	}

	var data []byte
	if data, err = json.Marshal(payload); err != nil {
		return
	}

	var rq *http.Request
	if rq, err = http.NewRequest("POST", uri.String(), bytes.NewBuffer(data)); err != nil {
		return
	}
	rq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.token))

	resource, err = s.executeRequest(rq)
	return
}

func (s *sync) deleteResource(resourceType string, resourceId string) (err error) {
	var uri *url.URL
	if uri, err = s.composeUrl(resourceType, resourceId); err != nil {
		return
	}

	var rq *http.Request
	if rq, err = http.NewRequest("DELETE", uri.String(), nil); err != nil {
		return
	}
	rq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.token))

	_, err = s.executeRequest(rq)
	return
}

func (s *sync) getResources(resourceType string, cb func(map[string]any)) (err error) {
	var uri *url.URL
	if uri, err = s.composeUrl(resourceType); err != nil {
		return
	}

	var startIndex int64 = 1
	var count = 500
	var attempt = 0
	for {
		attempt += 1
		if attempt > 20 {
			err = fmt.Errorf("get SCIM resource \"%s\" canceled", resourceType)
			return
		}
		var ruri = new(url.URL)
		*ruri = *uri
		ruri.Query().Add("startIndex", strconv.FormatInt(startIndex, 10))
		ruri.Query().Add("count", strconv.Itoa(count))

		var rq *http.Request
		if rq, err = http.NewRequest("GET", ruri.String(), nil); err != nil {
			return
		}
		rq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.token))

		var jo map[string]any
		if jo, err = s.executeRequest(rq); err != nil {
			return
		}
		var j any
		var ok bool
		if j, ok = jo["Resources"]; ok {
			var jr []any
			if jr, ok = j.([]any); ok {
				for _, j = range jr {
					var jor map[string]any
					if jor, ok = j.(map[string]any); ok {
						cb(jor)
					}
				}
			}
		}
		var itemsPerPage int64 = 0
		if itemsPerPage, ok = toInt64(jo["itemsPerPage"]); !ok {
			err = fmt.Errorf("response does not conform to SCIM specification: missing \"itemsPerPage\"")
			return
		}
		if startIndex, ok = toInt64(jo["startIndex"]); !ok {
			err = fmt.Errorf("response does not conform to SCIM specification: missing \"startIndex\"")
			return
		}
		startIndex += itemsPerPage

		var totalResults int64 = 0
		if totalResults, ok = toInt64(jo["totalResults"]); !ok {
			err = fmt.Errorf("response does not conform to SCIM specification: missing \"totalResults\"")
			return
		}
		if startIndex >= totalResults {
			return
		}
	}
}
