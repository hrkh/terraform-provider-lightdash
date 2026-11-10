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
	"testing"
)

func TestUserAttributeV1Request_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		request  UserAttributeV1Request
		expected string
	}{
		{
			name: "full",
			request: UserAttributeV1Request{
				Name:              "region",
				Description:       "The sales region",
				AttributeDefaults: []string{"jp"},
				Users:             []UserAttributeV1RequestUserValue{{UserUUID: "u", Values: []string{"eu", "jp"}}},
				Groups:            []UserAttributeV1RequestGroupValue{{GroupUUID: "g", Values: []string{"us"}}},
			},
			expected: `{"name":"region","description":"The sales region","attributeDefaults":["jp"],"users":[{"userUuid":"u","values":["eu","jp"]}],"groups":[{"groupUuid":"g","values":["us"]}]}`,
		},
		{
			// The API keeps the previous description when it is omitted and rejects a request
			// without the arrays, so an empty request must still send every field.
			name:     "empty",
			request:  UserAttributeV1Request{Name: "region"},
			expected: `{"name":"region","description":"","attributeDefaults":null,"users":[],"groups":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := json.Marshal(tt.request.normalized())
			if err != nil {
				t.Fatalf("failed to marshal JSON: %v", err)
			}
			if string(marshalled) != tt.expected {
				t.Errorf("unexpected JSON:\n got: %s\nwant: %s", marshalled, tt.expected)
			}
		})
	}
}

func TestCreateUserAttributeV1_validation(t *testing.T) {
	if _, err := CreateUserAttributeV1(nil, UserAttributeV1Request{Name: " "}); err == nil {
		t.Errorf("expected an error for an empty name")
	}
}
