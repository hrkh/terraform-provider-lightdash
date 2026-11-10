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

package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/models"
)

func TestGetUserAttributeResourceId(t *testing.T) {
	got := getUserAttributeResourceId("org-uuid", "attribute-uuid")
	expected := "organizations/org-uuid/user-attributes/attribute-uuid"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractUserAttributeResourceId(t *testing.T) {
	organizationUuid, userAttributeUuid, err := extractUserAttributeResourceId("organizations/org-uuid/user-attributes/attribute-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if organizationUuid != "org-uuid" || userAttributeUuid != "attribute-uuid" {
		t.Errorf("unexpected result: %s, %s", organizationUuid, userAttributeUuid)
	}
}

func TestExtractUserAttributeResourceId_invalid(t *testing.T) {
	invalidIds := []string{
		"",
		"attribute-uuid",
		"organizations/org-uuid/groups/group-uuid",
		"organizations/org-uuid/user-attributes/",
		"organizations/org-uuid/user-attributes/attribute-uuid/extra",
	}
	for _, id := range invalidIds {
		if _, _, err := extractUserAttributeResourceId(id); err == nil {
			t.Errorf("expected an error for %q", id)
		}
	}
}

func userAttributeTestValues(t *testing.T, elements ...string) types.Set {
	t.Helper()
	set, diags := stringSliceToStringSet(context.Background(), elements)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	return set
}

func TestValidateUserAttributeAssignments_valid(t *testing.T) {
	assignments := []userAttributeAssignment{
		{UUID: types.StringValue("group-1"), Values: userAttributeTestValues(t, "us")},
		{UUID: types.StringValue("group-2"), Values: userAttributeTestValues(t, "us", "eu")},
		{UUID: types.StringUnknown(), Values: types.SetUnknown(types.StringType)},
		{UUID: types.StringUnknown(), Values: userAttributeTestValues(t, "jp")},
	}
	if diags := validateUserAttributeAssignments(path.Root("groups"), "group", assignments); diags.HasError() {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestValidateUserAttributeAssignments_invalid(t *testing.T) {
	withNull := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("us"), types.StringNull()})
	assignments := []userAttributeAssignment{
		{UUID: types.StringValue("group-1"), Values: userAttributeTestValues(t, "us")},
		{UUID: types.StringValue("group-1"), Values: userAttributeTestValues(t, "eu")},
		{UUID: types.StringValue("group-2"), Values: userAttributeTestValues(t)},
		{UUID: types.StringValue("group-3"), Values: withNull},
		{UUID: types.StringValue("GROUP-4"), Values: userAttributeTestValues(t, "us")},
	}
	diags := validateUserAttributeAssignments(path.Root("groups"), "group", assignments)
	summaries := []string{}
	for _, d := range diags.Errors() {
		summaries = append(summaries, d.Summary())
	}
	expected := []string{"Empty values", "Null value", "Invalid group_uuid", "Duplicate group_uuid"}
	if !reflect.DeepEqual(summaries, expected) {
		t.Errorf("unexpected diagnostics: %v", summaries)
	}
}

func TestValidateUserAttributeAssignments_unknownUuid(t *testing.T) {
	// The message must not print the placeholder of a UUID that is only known after apply.
	assignments := []userAttributeAssignment{{UUID: types.StringUnknown(), Values: userAttributeTestValues(t)}}
	diags := validateUserAttributeAssignments(path.Root("groups"), "group", assignments)
	if len(diags.Errors()) != 1 {
		t.Fatalf("expected 1 error, got %v", diags)
	}
	if detail := diags.Errors()[0].Detail(); detail != "This group must have at least one value. Remove the group to assign no value." {
		t.Errorf("unexpected detail: %s", detail)
	}
}

func TestUserAttributeWriteErrorHint(t *testing.T) {
	existing := []models.UserAttribute{
		{OrganizationUUID: "org", UserAttributeUUID: "attr-1", Name: "region"},
		{OrganizationUUID: "org", UserAttributeUUID: "attr-2", Name: "team"},
	}
	tests := []struct {
		name      string
		attribute models.UserAttribute
		expected  string
	}{
		{
			name:      "create with a name that is taken",
			attribute: models.UserAttribute{Name: "region"},
			expected:  `A user attribute named "region" already exists. Choose another name, or manage the existing one by running terraform import with the ID organizations/org/user-attributes/attr-1`,
		},
		{
			name:      "rename to a name that is taken",
			attribute: models.UserAttribute{UserAttributeUUID: "attr-2", Name: "region"},
			expected:  `A user attribute named "region" already exists. Choose another name, or manage the existing one by running terraform import with the ID organizations/org/user-attributes/attr-1`,
		},
		{
			name:      "update of an attribute deleted outside of Terraform",
			attribute: models.UserAttribute{UserAttributeUUID: "attr-3", Name: "cost_center"},
			expected:  "The user attribute no longer exists in Lightdash. Run terraform apply again to create it.",
		},
		{
			name:      "unknown cause",
			attribute: models.UserAttribute{UserAttributeUUID: "attr-2", Name: "team"},
			expected:  "The Lightdash API did not report the cause. Check that every group_uuid is a group of the organization.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userAttributeWriteErrorHint(existing, tt.attribute); got != tt.expected {
				t.Errorf("unexpected hint:\n got: %s\nwant: %s", got, tt.expected)
			}
		})
	}
}

// User values are covered here, because the acceptance tests below don't assign them.
func TestUserAttributeResourceModel_roundTrip(t *testing.T) {
	ctx := context.Background()
	attribute := models.UserAttribute{
		OrganizationUUID:  "org-uuid",
		UserAttributeUUID: "attribute-uuid",
		Name:              "region",
		Description:       "The sales region",
		AttributeDefaults: []string{"jp"},
		Users:             []models.UserAttributeValue{{UUID: "user-1", Values: []string{"eu", "jp"}}},
		Groups: []models.UserAttributeValue{
			{UUID: "group-1", Values: []string{"us"}},
			{UUID: "group-2", Values: []string{"eu"}},
		},
	}

	model, diags := userAttributeToResourceModel(ctx, attribute)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if model.ID.ValueString() != "organizations/org-uuid/user-attributes/attribute-uuid" {
		t.Errorf("unexpected ID: %s", model.ID.ValueString())
	}
	if len(model.Groups.Elements()) != 2 || len(model.Users.Elements()) != 1 || len(model.AttributeDefaults.Elements()) != 1 {
		t.Errorf("unexpected model: %+v", model)
	}

	converted, diags := userAttributeFromResourceModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	// The UUIDs are not part of the request body.
	expected := attribute
	expected.OrganizationUUID = ""
	expected.UserAttributeUUID = ""
	sortUserAttributeForTest(&converted)
	if !reflect.DeepEqual(converted, expected) {
		t.Errorf("unexpected user attribute:\n got: %+v\nwant: %+v", converted, expected)
	}
}

func TestUserAttributeToResourceModel_empty(t *testing.T) {
	ctx := context.Background()
	attribute := models.UserAttribute{
		OrganizationUUID:  "org-uuid",
		UserAttributeUUID: "attribute-uuid",
		Name:              "region",
	}

	model, diags := userAttributeToResourceModel(ctx, attribute)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if model.Description.IsNull() || model.Description.ValueString() != "" {
		t.Errorf("expected an empty description, got %s", model.Description)
	}
	if !model.AttributeDefaults.IsNull() {
		t.Errorf("expected null attribute_defaults, got %s", model.AttributeDefaults)
	}
	if model.Groups.IsNull() || len(model.Groups.Elements()) != 0 {
		t.Errorf("expected empty groups, got %s", model.Groups)
	}
	if model.Users.IsNull() || len(model.Users.Elements()) != 0 {
		t.Errorf("expected empty users, got %s", model.Users)
	}

	// The API requires `users` and `groups`, so they must go back as empty arrays and not as null.
	converted, diags := userAttributeFromResourceModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if converted.AttributeDefaults != nil {
		t.Errorf("expected nil defaults, got %v", converted.AttributeDefaults)
	}
	if converted.Users == nil || converted.Groups == nil {
		t.Errorf("expected non-nil users and groups, got %#v", converted)
	}
}

func TestUserAttributeToResourceModel_duplicateValues(t *testing.T) {
	ctx := context.Background()
	attribute := models.UserAttribute{
		OrganizationUUID:  "org-uuid",
		UserAttributeUUID: "attribute-uuid",
		Name:              "region",
		Groups:            []models.UserAttributeValue{{UUID: "group-1", Values: []string{"b", "a", "a"}}},
	}

	model, diags := userAttributeToResourceModel(ctx, attribute)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	converted, diags := userAttributeFromResourceModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	sortUserAttributeForTest(&converted)
	expectedGroups := []models.UserAttributeValue{{UUID: "group-1", Values: []string{"a", "b"}}}
	if !reflect.DeepEqual(converted.Groups, expectedGroups) {
		t.Errorf("unexpected groups: %+v", converted.Groups)
	}
}

func TestUserAttributeAssignmentsFromSet_unknown(t *testing.T) {
	if got := userAttributeAssignmentsFromSet(types.SetNull(userAttributeGroupValueObjectType), "group_uuid"); len(got) != 0 {
		t.Errorf("expected no assignments for a null set, got %v", got)
	}
	if got := userAttributeAssignmentsFromSet(types.SetUnknown(userAttributeGroupValueObjectType), "group_uuid"); len(got) != 0 {
		t.Errorf("expected no assignments for an unknown set, got %v", got)
	}

	set := types.SetValueMust(userAttributeGroupValueObjectType, []attr.Value{
		types.ObjectUnknown(userAttributeGroupValueObjectType.AttrTypes),
		types.ObjectValueMust(userAttributeGroupValueObjectType.AttrTypes, map[string]attr.Value{
			"group_uuid": types.StringUnknown(),
			"values":     types.SetUnknown(types.StringType),
		}),
	})
	got := userAttributeAssignmentsFromSet(set, "group_uuid")
	if len(got) != 1 || !got[0].UUID.IsUnknown() || !got[0].Values.IsUnknown() {
		t.Errorf("unexpected assignments: %v", got)
	}
}

func sortUserAttributeForTest(attribute *models.UserAttribute) {
	sort.Strings(attribute.AttributeDefaults)
	for i := range attribute.Users {
		sort.Strings(attribute.Users[i].Values)
	}
	for i := range attribute.Groups {
		sort.Strings(attribute.Groups[i].Values)
	}
	sort.Slice(attribute.Groups, func(i, j int) bool {
		return attribute.Groups[i].UUID < attribute.Groups[j].UUID
	})
}

// The acceptance tests assign values only to groups created by the tests.
// They don't assign values to users, because that changes the data access of real users.

func TestAccUserAttributeResource_create(t *testing.T) {
	if !isIntegrationTestMode() {
		t.Skip("Skipping acceptance test for resource_lightdash_user_attribute")
	}

	providerConfig, err := getProviderConfig()
	if err != nil {
		t.Fatalf("Failed to get providerConfig: %v", err)
	}

	createConfig010, err := ReadAccTestResource([]string{"resources", "lightdash_user_attribute", "create", "010_create.tf"})
	if err != nil {
		t.Fatalf("Failed to get createConfig: %v", err)
	}
	updateConfig020, err := ReadAccTestResource([]string{"resources", "lightdash_user_attribute", "create", "020_update.tf"})
	if err != nil {
		t.Fatalf("Failed to get updateConfig: %v", err)
	}
	removeConfig030, err := ReadAccTestResource([]string{"resources", "lightdash_user_attribute", "create", "030_remove.tf"})
	if err != nil {
		t.Fatalf("Failed to get removeConfig: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + createConfig010,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "name", "zzz_acc_test_user_attribute_create"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "description", "Acceptance Test - create"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "attribute_defaults.#", "1"),
					resource.TestCheckTypeSetElemAttr("lightdash_user_attribute.test", "attribute_defaults.*", "default"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "groups.#", "1"),
					resource.TestCheckTypeSetElemAttrPair("lightdash_user_attribute.test", "groups.*.group_uuid", "lightdash_group.test_group1", "group_uuid"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "users.#", "0"),
					resource.TestCheckResourceAttrSet("lightdash_user_attribute.test", "user_attribute_uuid"),
					resource.TestCheckResourceAttrPair("lightdash_user_attribute.test", "organization_uuid", "data.lightdash_organization.test", "organization_uuid"),
				),
			},
			{
				Config: providerConfig + updateConfig020,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "name", "zzz_acc_test_user_attribute_update"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "description", "Acceptance Test - update"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "attribute_defaults.#", "2"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "groups.#", "2"),
					resource.TestCheckTypeSetElemAttrPair("lightdash_user_attribute.test", "groups.*.group_uuid", "lightdash_group.test_group1", "group_uuid"),
					resource.TestCheckTypeSetElemAttrPair("lightdash_user_attribute.test", "groups.*.group_uuid", "lightdash_group.test_group2", "group_uuid"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "users.#", "0"),
				),
			},
			{
				Config: providerConfig + removeConfig030,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "name", "zzz_acc_test_user_attribute_update"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "description", ""),
					resource.TestCheckNoResourceAttr("lightdash_user_attribute.test", "attribute_defaults.#"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "groups.#", "0"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "users.#", "0"),
				),
			},
		},
	})
}

func TestAccUserAttributeResource_import(t *testing.T) {
	if !isIntegrationTestMode() {
		t.Skip("Skipping acceptance test for resource_lightdash_user_attribute")
	}

	providerConfig, err := getProviderConfig()
	if err != nil {
		t.Fatalf("Failed to get providerConfig: %v", err)
	}

	importConfig010, err := ReadAccTestResource([]string{"resources", "lightdash_user_attribute", "import", "010_import.tf"})
	if err != nil {
		t.Fatalf("Failed to get importConfig: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + importConfig010,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "name", "zzz_acc_test_user_attribute_import"),
					resource.TestCheckResourceAttr("lightdash_user_attribute.test", "groups.#", "1"),
				),
			},
			{
				Config:            providerConfig + importConfig010,
				ResourceName:      "lightdash_user_attribute.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					res, ok := state.RootModule().Resources["lightdash_user_attribute.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state for import")
					}
					organizationUuid, ok := res.Primary.Attributes["organization_uuid"]
					if !ok || organizationUuid == "" {
						return "", fmt.Errorf("organization_uuid attribute not present in state")
					}
					userAttributeUuid, ok := res.Primary.Attributes["user_attribute_uuid"]
					if !ok || userAttributeUuid == "" {
						return "", fmt.Errorf("user_attribute_uuid attribute not present in state")
					}
					return getUserAttributeResourceId(organizationUuid, userAttributeUuid), nil
				},
			},
		},
	})
}
