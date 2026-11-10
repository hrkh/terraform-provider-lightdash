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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/api"
)

// UserAttributeV1UserValue is the values of a user attribute assigned to a user.
type UserAttributeV1UserValue struct {
	UserUUID string   `json:"userUuid"`
	Email    string   `json:"email"`
	Values   []string `json:"values"`
}

// UserAttributeV1GroupValue is the values of a user attribute assigned to a group.
type UserAttributeV1GroupValue struct {
	GroupUUID string   `json:"groupUuid"`
	Values    []string `json:"values"`
}

// UserAttributeV1 is the user attribute returned by the Lightdash API.
// The deprecated single `value` and `attributeDefault` in the response are not read.
// Lightdash versions that only have them can't take the `values` that this provider writes,
// so reading them would not make those versions usable.
type UserAttributeV1 struct {
	UUID             string                      `json:"uuid"`
	OrganizationUUID string                      `json:"organizationUuid"`
	Name             string                      `json:"name"`
	Description      *string                     `json:"description,omitempty"`
	CreatedAt        string                      `json:"createdAt"`
	Users            []UserAttributeV1UserValue  `json:"users"`
	Groups           []UserAttributeV1GroupValue `json:"groups"`
	// AttributeDefaults is nil when the attribute has no default values.
	AttributeDefaults []string `json:"attributeDefaults"`
}

type GetUserAttributesV1Response struct {
	Results []UserAttributeV1 `json:"results"`
	Status  string            `json:"status"`
}

// GetUserAttributesV1 lists all user attributes in the organization of the authenticated user.
// The Lightdash API has no endpoint to get a single user attribute.
func GetUserAttributesV1(c *api.Client) ([]UserAttributeV1, error) {
	path := fmt.Sprintf("%s/api/v1/org/attributes", c.HostUrl)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request for listing user attributes: %w", err)
	}

	body, err := c.DoRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for listing user attributes: %w", err)
	}

	response := GetUserAttributesV1Response{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body for listing user attributes: %w", err)
	}

	return response.Results, nil
}
