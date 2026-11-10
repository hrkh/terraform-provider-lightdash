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

package services

import (
	"errors"
	"reflect"
	"testing"

	apiv1 "github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/api/v1"
	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/models"
)

func TestToUserAttributeModel(t *testing.T) {
	description := "The sales region"
	attribute := toUserAttributeModel(apiv1.UserAttributeV1{
		UUID:              "attr-2",
		OrganizationUUID:  "org",
		Name:              "region",
		Description:       &description,
		AttributeDefaults: []string{"jp"},
		Users:             []apiv1.UserAttributeV1UserValue{{UserUUID: "user-1", Email: "a@example.com", Values: []string{"eu", "jp"}}},
		Groups:            []apiv1.UserAttributeV1GroupValue{{GroupUUID: "group-1", Values: []string{"b", "a", "a"}}},
	})
	expected := models.UserAttribute{
		OrganizationUUID:  "org",
		UserAttributeUUID: "attr-2",
		Name:              "region",
		Description:       "The sales region",
		AttributeDefaults: []string{"jp"},
		Users:             []models.UserAttributeValue{{UUID: "user-1", Values: []string{"eu", "jp"}}},
		// The values are kept as the API stores them, including duplicates.
		Groups: []models.UserAttributeValue{{UUID: "group-1", Values: []string{"b", "a", "a"}}},
	}
	if !reflect.DeepEqual(attribute, expected) {
		t.Errorf("unexpected user attribute:\n got: %+v\nwant: %+v", attribute, expected)
	}

	noValues := toUserAttributeModel(apiv1.UserAttributeV1{UUID: "attr-1", OrganizationUUID: "org", Name: "team"})
	if noValues.Description != "" || noValues.AttributeDefaults != nil {
		t.Errorf("expected no description and no defaults, got %+v", noValues)
	}
	if noValues.Users == nil || noValues.Groups == nil || len(noValues.Users) != 0 || len(noValues.Groups) != 0 {
		t.Errorf("expected empty non-nil users and groups, got %+v", noValues)
	}
}

func TestCheckUserAttributeOrganization(t *testing.T) {
	if err := checkUserAttributeOrganization("org-1", "org-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// A token of another organization must not look like a missing user attribute.
	err := checkUserAttributeOrganization("org-1", "org-2")
	if !errors.Is(err, ErrUserAttributeOrganizationMismatch) {
		t.Errorf("expected ErrUserAttributeOrganizationMismatch, got %v", err)
	}
	if errors.Is(err, ErrUserAttributeNotFound) {
		t.Errorf("a mismatch must not be reported as not found")
	}
}
