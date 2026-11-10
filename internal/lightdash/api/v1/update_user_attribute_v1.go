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

type UpdateUserAttributeV1Response struct {
	Results UserAttributeV1 `json:"results"`
	Status  string          `json:"status"`
}

// UpdateUserAttributeV1 replaces the user attribute, including all user and group values.
func UpdateUserAttributeV1(c *api.Client, userAttributeUuid string, request UserAttributeV1Request) (*UserAttributeV1, error) {
	if strings.TrimSpace(userAttributeUuid) == "" {
		return nil, fmt.Errorf("user attribute UUID is empty")
	}
	if strings.TrimSpace(request.Name) == "" {
		return nil, fmt.Errorf("user attribute name is empty")
	}

	marshalled, err := json.Marshal(request.normalized())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user attribute: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/org/attributes/%s", c.HostUrl, userAttributeUuid)
	req, err := http.NewRequest("PUT", path, bytes.NewReader(marshalled))
	if err != nil {
		return nil, fmt.Errorf("failed to create new request for updating user attribute: %w", err)
	}

	body, err := c.DoRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for updating user attribute: %w", err)
	}

	response := UpdateUserAttributeV1Response{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body for updating user attribute: %w", err)
	}
	if response.Results.UUID == "" {
		return nil, fmt.Errorf("user attribute UUID is missing in the response")
	}

	return &response.Results, nil
}
