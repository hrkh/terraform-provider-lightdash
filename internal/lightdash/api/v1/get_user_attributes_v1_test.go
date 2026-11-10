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
	"reflect"
	"testing"
)

func TestGetUserAttributesV1Response_UnmarshalJSON(t *testing.T) {
	// Captured from Lightdash 3.194.3.
	jsonStr := `{
		"status": "ok",
		"results": [
			{
				"uuid": "d47a8687-8bc7-4bc4-950d-0031fbfb6e22",
				"createdAt": "2026-11-10T10:43:53.657Z",
				"name": "region",
				"organizationUuid": "f830e6f3-456d-40b8-901d-7d9b870b3477",
				"description": "d1",
				"attributeDefaults": ["x", "y"],
				"attributeDefault": "x",
				"users": [
					{
						"userUuid": "fbc7c1f6-a077-40aa-9dad-149f21b79af1",
						"values": ["u1", "u2"],
						"value": "u1",
						"email": "admin@example.com"
					}
				],
				"groups": [
					{
						"groupUuid": "5e6470be-9bc2-48b3-ba04-526710565190",
						"values": ["g1"],
						"value": "g1"
					}
				]
			},
			{
				"uuid": "0b0e6b1e-2f0d-4a8e-8a53-7d3f6b4d2a11",
				"createdAt": "2026-11-10T10:43:53.657Z",
				"name": "no_values",
				"organizationUuid": "f830e6f3-456d-40b8-901d-7d9b870b3477",
				"attributeDefaults": null,
				"attributeDefault": null,
				"users": [],
				"groups": []
			}
		]
	}`

	var response GetUserAttributesV1Response
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(response.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(response.Results))
	}

	full := response.Results[0]
	if full.UUID != "d47a8687-8bc7-4bc4-950d-0031fbfb6e22" {
		t.Errorf("unexpected UUID: %s", full.UUID)
	}
	if full.OrganizationUUID != "f830e6f3-456d-40b8-901d-7d9b870b3477" {
		t.Errorf("unexpected OrganizationUUID: %s", full.OrganizationUUID)
	}
	if full.Name != "region" {
		t.Errorf("unexpected Name: %s", full.Name)
	}
	if full.Description == nil || *full.Description != "d1" {
		t.Errorf("unexpected Description: %v", full.Description)
	}
	if got := full.AttributeDefaults; !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("unexpected attribute defaults: %v", got)
	}
	if len(full.Users) != 1 || full.Users[0].UserUUID != "fbc7c1f6-a077-40aa-9dad-149f21b79af1" {
		t.Fatalf("unexpected Users: %+v", full.Users)
	}
	if got := full.Users[0].Values; !reflect.DeepEqual(got, []string{"u1", "u2"}) {
		t.Errorf("unexpected user values: %v", got)
	}
	if len(full.Groups) != 1 || full.Groups[0].GroupUUID != "5e6470be-9bc2-48b3-ba04-526710565190" {
		t.Fatalf("unexpected Groups: %+v", full.Groups)
	}
	if got := full.Groups[0].Values; !reflect.DeepEqual(got, []string{"g1"}) {
		t.Errorf("unexpected group values: %v", got)
	}

	empty := response.Results[1]
	if empty.Description != nil {
		t.Errorf("expected Description to be nil, got %v", *empty.Description)
	}
	if got := empty.AttributeDefaults; got != nil {
		t.Errorf("expected attribute defaults to be nil, got %v", got)
	}
	if len(empty.Users) != 0 || len(empty.Groups) != 0 {
		t.Errorf("expected no users and groups, got %+v and %+v", empty.Users, empty.Groups)
	}
}
