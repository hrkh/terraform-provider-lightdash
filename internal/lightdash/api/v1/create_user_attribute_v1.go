// Copyright 2023 Ubie, inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/api"
)

type UserAttributeV1RequestUserValue struct {
	UserUUID string   `json:"userUuid"`
	Values   []string `json:"values"`
}

type UserAttributeV1RequestGroupValue struct {
	GroupUUID string   `json:"groupUuid"`
	Values    []string `json:"values"`
}

// UserAttributeV1Request is the request body to create or update a user attribute.
// The update endpoint replaces the whole attribute, including all user and group values.
// The deprecated single `value` and `attributeDefault` are not sent, because they can hold
// only one value. Lightdash 0.3243.0 or later is therefore required.
// No field has `omitempty`, because the API keeps the previous description when it is
// missing and rejects a request without `users` or `groups`.
type UserAttributeV1Request struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// AttributeDefaults is sent as `null` when nil, which means no default values.
	AttributeDefaults []string `json:"attributeDefaults"`
	// Users and Groups are required by the API and are always sent as arrays.
	Users  []UserAttributeV1RequestUserValue  `json:"users"`
	Groups []UserAttributeV1RequestGroupValue `json:"groups"`
}

// normalized returns a copy of the request whose required arrays are never nil,
// so that they are marshalled as `[]` instead of `null`.
func (r UserAttributeV1Request) normalized() UserAttributeV1Request {
	if r.Users == nil {
		r.Users = []UserAttributeV1RequestUserValue{}
	}
	if r.Groups == nil {
		r.Groups = []UserAttributeV1RequestGroupValue{}
	}
	return r
}

type CreateUserAttributeV1Response struct {
	Results UserAttributeV1 `json:"results"`
	Status  string          `json:"status"`
}

func CreateUserAttributeV1(c *api.Client, request UserAttributeV1Request) (*UserAttributeV1, error) {
	if strings.TrimSpace(request.Name) == "" {
		return nil, fmt.Errorf("user attribute name is empty")
	}

	marshalled, err := json.Marshal(request.normalized())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user attribute: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/org/attributes", c.HostUrl)
	req, err := http.NewRequest("POST", path, bytes.NewReader(marshalled))
	if err != nil {
		return nil, fmt.Errorf("failed to create new request for creating user attribute: %w", err)
	}

	body, err := c.DoRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for creating user attribute: %w", err)
	}

	response := CreateUserAttributeV1Response{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body for creating user attribute: %w", err)
	}
	if response.Results.UUID == "" {
		return nil, fmt.Errorf("user attribute UUID is missing in the response")
	}

	return &response.Results, nil
}
