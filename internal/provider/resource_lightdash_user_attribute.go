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
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/api"
	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/models"
	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/services"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &userAttributeResource{}
	_ resource.ResourceWithConfigure      = &userAttributeResource{}
	_ resource.ResourceWithImportState    = &userAttributeResource{}
	_ resource.ResourceWithValidateConfig = &userAttributeResource{}
)

var (
	userAttributeGroupValueObjectType = types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"group_uuid": types.StringType,
			"values":     types.SetType{ElemType: types.StringType},
		},
	}
	userAttributeUserValueObjectType = types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"user_uuid": types.StringType,
			"values":    types.SetType{ElemType: types.StringType},
		},
	}
)

func NewUserAttributeResource() resource.Resource {
	return &userAttributeResource{}
}

// userAttributeResource manages a whole user attribute, including its group and user values.
// The values are not split into resources per group or user, because the Lightdash API can only
// replace the whole attribute. Each of those resources would have to read, modify and write all
// values, so parallel applies would overwrite each other.
type userAttributeResource struct {
	client  *api.Client
	service *services.UserAttributesService
}

// userAttributeResourceModel describes the resource data model.
type userAttributeResourceModel struct {
	ID                types.String `tfsdk:"id"`
	OrganizationUUID  types.String `tfsdk:"organization_uuid"`
	UserAttributeUUID types.String `tfsdk:"user_attribute_uuid"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	AttributeDefaults types.Set    `tfsdk:"attribute_defaults"`
	Groups            types.Set    `tfsdk:"groups"`
	Users             types.Set    `tfsdk:"users"`
}

func (r *userAttributeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute"
}

func (r *userAttributeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	markdownDescription, err := readMarkdownDescription(ctx, "internal/provider/docs/resources/resource_user_attribute.md")
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read markdown description",
			fmt.Sprintf("Unable to read schema markdown description file: %s", err.Error()),
		)
		return
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: markdownDescription,
		Description:         "Manages a Lightdash user attribute and its values for groups and users",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The resource identifier. It is computed as `organizations/<organization_uuid>/user-attributes/<user_attribute_uuid>`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the Lightdash organization that the user attribute belongs to.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_attribute_uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the Lightdash user attribute.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the user attribute. It is the name referenced from dbt, such as `required_attributes` and `${lightdash.attributes.<name>}`.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the user attribute. It defaults to an empty string.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"attribute_defaults": schema.SetAttribute{
				MarkdownDescription: "The default values for users who don't have their own value in this user attribute. Omit it to have no default values. It must not be an empty set.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"groups": schema.SetNestedAttribute{
				MarkdownDescription: "The values of the user attribute assigned to groups. This is authoritative, so group values that are not in the configuration are removed.",
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(types.SetValueMust(userAttributeGroupValueObjectType, []attr.Value{})),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"group_uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the Lightdash group. It must be written in lowercase.",
							Required:            true,
						},
						"values": schema.SetAttribute{
							MarkdownDescription: "The values of the user attribute for the group. At least one value is required.",
							ElementType:         types.StringType,
							Required:            true,
						},
					},
				},
			},
			"users": schema.SetNestedAttribute{
				MarkdownDescription: "The values of the user attribute assigned to users. This is authoritative, so user values that are not in the configuration are removed.",
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(types.SetValueMust(userAttributeUserValueObjectType, []attr.Value{})),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"user_uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the Lightdash user, who must be a member of the organization. It must be written in lowercase.",
							Required:            true,
						},
						"values": schema.SetAttribute{
							MarkdownDescription: "The values of the user attribute for the user. At least one value is required.",
							ElementType:         types.StringType,
							Required:            true,
						},
					},
				},
			},
		},
	}
}

func (r *userAttributeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
	r.service = services.NewUserAttributesService(client)
}

// ValidateConfig holds the rules that the schema can't express.
// They are not written with terraform-plugin-framework-validators, because that would add
// a dependency for a few rules, and a repeated UUID has to be found across set elements anyway.
// The Lightdash API is not relied on either, because it accepts and stores empty values.
// Every rule skips values that are only known after apply.
func (r *userAttributeResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config userAttributeResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The shared ValidateNonEmptyString is not used, because it also rejects a name
	// that is only known after apply, such as a reference to another resource.
	if !config.Name.IsNull() && !config.Name.IsUnknown() && strings.TrimSpace(config.Name.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Empty name", "name must not be empty.")
	}

	if isKnownEmptySet(config.AttributeDefaults) {
		resp.Diagnostics.AddAttributeError(
			path.Root("attribute_defaults"),
			"Empty attribute_defaults",
			"attribute_defaults must have at least one value. Omit the attribute to have no default values.",
		)
	}
	if hasNullElement(config.AttributeDefaults) {
		resp.Diagnostics.AddAttributeError(path.Root("attribute_defaults"), "Null value", "attribute_defaults must not contain null.")
	}

	resp.Diagnostics.Append(validateUserAttributeAssignments(path.Root("groups"), "group", userAttributeAssignmentsFromSet(config.Groups, "group_uuid"))...)
	resp.Diagnostics.Append(validateUserAttributeAssignments(path.Root("users"), "user", userAttributeAssignmentsFromSet(config.Users, "user_uuid"))...)
}

// userAttributeAssignment is the values assigned to a group or a user in the configuration.
type userAttributeAssignment struct {
	// Element is the set element that the assignment was read from. It locates diagnostics.
	Element attr.Value
	UUID    types.String
	Values  types.Set
}

// validateUserAttributeAssignments checks the `groups` or `users` set at attributePath.
func validateUserAttributeAssignments(attributePath path.Path, kind string, assignments []userAttributeAssignment) diag.Diagnostics {
	var diags diag.Diagnostics
	uuids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		elementPath := attributePath
		if assignment.Element != nil {
			elementPath = attributePath.AtSetValue(assignment.Element)
		}
		uuidIsKnown := !assignment.UUID.IsNull() && !assignment.UUID.IsUnknown()
		label := fmt.Sprintf("This %s", kind)
		if uuidIsKnown {
			label = fmt.Sprintf("The %s %q", kind, assignment.UUID.ValueString())
		}

		if isKnownEmptySet(assignment.Values) {
			diags.AddAttributeError(elementPath, "Empty values",
				fmt.Sprintf("%s must have at least one value. Remove the %s to assign no value.", label, kind))
		}
		if hasNullElement(assignment.Values) {
			diags.AddAttributeError(elementPath, "Null value",
				fmt.Sprintf("%s must not have null in its values.", label))
		}
		if !uuidIsKnown {
			continue
		}
		uuid := assignment.UUID.ValueString()
		// Lightdash returns UUIDs in lowercase, so any other spelling in the plan
		// would differ from the result of the apply.
		if uuid != strings.ToLower(uuid) {
			diags.AddAttributeError(elementPath, fmt.Sprintf("Invalid %s_uuid", kind),
				fmt.Sprintf("%s must be written in lowercase, as Lightdash returns it.", label))
		}
		uuids = append(uuids, uuid)
	}
	for _, duplicate := range findDuplicateStrings(uuids) {
		diags.AddAttributeError(attributePath, fmt.Sprintf("Duplicate %s_uuid", kind),
			fmt.Sprintf("The %s %q appears more than once. Merge its values into a single element.", kind, duplicate))
	}
	return diags
}

// checkUsersAreOrganizationMembers rejects users who are not members of the organization.
// The Lightdash API doesn't do it. It fails with a generic 500 for an unknown user and
// accepts a user of another organization.
func (r *userAttributeResource) checkUsersAreOrganizationMembers(ctx context.Context, users []models.UserAttributeValue) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(users) == 0 {
		return diags
	}

	members, err := services.GetOrganizationMembersService(r.client).GetOrganizationMembersByCache(ctx)
	if err != nil {
		diags.AddError("Error getting organization members", "Could not check the users of the user attribute: "+err.Error())
		return diags
	}
	memberUuids := make([]string, 0, len(members))
	for _, member := range members {
		memberUuids = append(memberUuids, member.UserUUID)
	}
	userUuids := make([]string, 0, len(users))
	for _, user := range users {
		userUuids = append(userUuids, user.UUID)
	}
	for _, missing := range subtractStringList(userUuids, memberUuids) {
		diags.AddAttributeError(path.Root("users"), "Unknown user_uuid",
			fmt.Sprintf("The user %q is not a member of the organization.", missing))
	}
	return diags
}

// explainWriteError adds what can be found out about a failed create or update,
// because the Lightdash API answers most of them with a generic 500.
func (r *userAttributeResource) explainWriteError(ctx context.Context, attribute models.UserAttribute, err error) string {
	existing, listErr := r.service.List(ctx)
	if listErr != nil {
		return err.Error()
	}
	return err.Error() + "\n\n" + userAttributeWriteErrorHint(existing, attribute)
}

// userAttributeWriteErrorHint guesses why the attribute could not be written.
// An empty UserAttributeUUID means that the attribute was being created.
func userAttributeWriteErrorHint(existing []models.UserAttribute, attribute models.UserAttribute) string {
	found := attribute.UserAttributeUUID == ""
	for _, other := range existing {
		if other.UserAttributeUUID == attribute.UserAttributeUUID {
			found = true
			continue
		}
		if other.Name == attribute.Name {
			return fmt.Sprintf(
				"A user attribute named %q already exists. Choose another name, or manage the existing one by running terraform import with the ID %s",
				attribute.Name, getUserAttributeResourceId(other.OrganizationUUID, other.UserAttributeUUID),
			)
		}
	}
	if !found {
		return "The user attribute no longer exists in Lightdash. Run terraform apply again to create it."
	}
	return "The Lightdash API did not report the cause. Check that every group_uuid is a group of the organization."
}

func (r *userAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attribute, diags := userAttributeFromResourceModel(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.checkUsersAreOrganizationMembers(ctx, attribute.Users)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.service.Create(ctx, attribute)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating user attribute",
			"Could not create user attribute, unexpected error: "+r.explainWriteError(ctx, attribute, err),
		)
		return
	}
	tflog.Info(ctx, fmt.Sprintf("Created user attribute %s", created.UserAttributeUUID))

	state, diags := userAttributeToResourceModel(ctx, *created)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attribute, err := r.service.GetByUuid(ctx, state.OrganizationUUID.ValueString(), state.UserAttributeUUID.ValueString())
	if err != nil {
		// The resource is only dropped from the state when its own organization no longer has it.
		// A token of another organization gets an error instead, because dropping the resource
		// would recreate it in that organization and leave the original unmanaged.
		if errors.Is(err, services.ErrUserAttributeNotFound) {
			tflog.Warn(ctx, fmt.Sprintf("User attribute %s no longer exists. Removing it from the state.", state.UserAttributeUUID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading user attribute",
			"Could not read user attribute ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	newState, diags := userAttributeToResourceModel(ctx, *attribute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *userAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userAttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attribute, diags := userAttributeFromResourceModel(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	attribute.UserAttributeUUID = state.UserAttributeUUID.ValueString()

	resp.Diagnostics.Append(r.checkUsersAreOrganizationMembers(ctx, attribute.Users)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API replaces the whole user attribute, so the plan is sent as it is.
	updated, err := r.service.Update(ctx, attribute)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating user attribute",
			fmt.Sprintf("Could not update user attribute with UUID %q, unexpected error: %s", attribute.UserAttributeUUID, r.explainWriteError(ctx, attribute, err)),
		)
		return
	}

	newState, diags := userAttributeToResourceModel(ctx, *updated)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *userAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userAttributeUuid := state.UserAttributeUUID.ValueString()
	tflog.Info(ctx, fmt.Sprintf("Deleting user attribute %s", userAttributeUuid))
	if err := r.service.Delete(ctx, state.OrganizationUUID.ValueString(), userAttributeUuid); err != nil {
		resp.Diagnostics.AddError(
			"Error deleting user attribute",
			"Could not delete user attribute, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *userAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Lightdash returns UUIDs in lowercase, so the ID is matched in lowercase too.
	organizationUuid, userAttributeUuid, err := extractUserAttributeResourceId(strings.ToLower(req.ID))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error extracting resource ID",
			"Could not extract resource ID, unexpected error: "+err.Error(),
		)
		return
	}
	// Read fetches the attribute again right after the import. It is still fetched here
	// and not left to Read, so that an ID with another organization fails the import.
	attribute, err := r.service.GetByUuid(ctx, organizationUuid, userAttributeUuid)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting user attribute",
			fmt.Sprintf("Could not get user attribute with UUID %s, unexpected error: %s", userAttributeUuid, err.Error()),
		)
		return
	}

	state, diags := userAttributeToResourceModel(ctx, *attribute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// userAttributeAssignmentsFromSet reads the `groups` or `users` set, whose elements are
// objects with the UUID attribute named uuidKey and `values`.
// A null or unknown set and unknown elements have no assignments to read.
func userAttributeAssignmentsFromSet(set types.Set, uuidKey string) []userAttributeAssignment {
	assignments := make([]userAttributeAssignment, 0, len(set.Elements()))
	for _, element := range set.Elements() {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			continue
		}
		uuid, _ := object.Attributes()[uuidKey].(types.String)
		values, _ := object.Attributes()["values"].(types.Set)
		assignments = append(assignments, userAttributeAssignment{Element: element, UUID: uuid, Values: values})
	}
	return assignments
}

func userAttributeValuesFromSet(ctx context.Context, set types.Set, uuidKey string) ([]models.UserAttributeValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	assignments := userAttributeAssignmentsFromSet(set, uuidKey)
	result := make([]models.UserAttributeValue, 0, len(assignments))
	for _, assignment := range assignments {
		values, d := stringSetToStringSlice(ctx, assignment.Values)
		diags.Append(d...)
		result = append(result, models.UserAttributeValue{UUID: assignment.UUID.ValueString(), Values: values})
	}
	return result, diags
}

func userAttributeValuesToSet(ctx context.Context, objectType types.ObjectType, uuidKey string, values []models.UserAttributeValue) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		// The Lightdash API stores duplicate values as they are, and a set must hold each value once.
		// The shared stringSliceToStringSet is not changed for that, so that the other resources
		// using it keep their behavior.
		valuesSet, d := stringSliceToStringSet(ctx, uniqueStrings(value.Values))
		diags.Append(d...)
		object, d := types.ObjectValue(objectType.AttrTypes, map[string]attr.Value{
			uuidKey:  types.StringValue(value.UUID),
			"values": valuesSet,
		})
		diags.Append(d...)
		elements = append(elements, object)
	}
	set, d := types.SetValue(objectType, elements)
	diags.Append(d...)
	return set, diags
}

// userAttributeFromResourceModel converts the planned resource model
// to the user attribute to send to the API.
func userAttributeFromResourceModel(ctx context.Context, model userAttributeResourceModel) (models.UserAttribute, diag.Diagnostics) {
	var diags diag.Diagnostics
	attribute := models.UserAttribute{
		Name:        model.Name.ValueString(),
		Description: model.Description.ValueString(),
	}

	if !model.AttributeDefaults.IsNull() && !model.AttributeDefaults.IsUnknown() {
		defaults, d := stringSetToStringSlice(ctx, model.AttributeDefaults)
		diags.Append(d...)
		attribute.AttributeDefaults = defaults
	}

	groups, d := userAttributeValuesFromSet(ctx, model.Groups, "group_uuid")
	diags.Append(d...)
	attribute.Groups = groups

	users, d := userAttributeValuesFromSet(ctx, model.Users, "user_uuid")
	diags.Append(d...)
	attribute.Users = users

	return attribute, diags
}

// userAttributeToResourceModel converts the user attribute from the API to the resource model.
func userAttributeToResourceModel(ctx context.Context, attribute models.UserAttribute) (userAttributeResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := userAttributeResourceModel{
		ID:                types.StringValue(getUserAttributeResourceId(attribute.OrganizationUUID, attribute.UserAttributeUUID)),
		OrganizationUUID:  types.StringValue(attribute.OrganizationUUID),
		UserAttributeUUID: types.StringValue(attribute.UserAttributeUUID),
		Name:              types.StringValue(attribute.Name),
		Description:       types.StringValue(attribute.Description),
		AttributeDefaults: types.SetNull(types.StringType),
	}

	// The API stores empty default values as no default values.
	if len(attribute.AttributeDefaults) > 0 {
		defaults, d := stringSliceToStringSet(ctx, uniqueStrings(attribute.AttributeDefaults))
		diags.Append(d...)
		model.AttributeDefaults = defaults
	}

	groups, d := userAttributeValuesToSet(ctx, userAttributeGroupValueObjectType, "group_uuid", attribute.Groups)
	diags.Append(d...)
	model.Groups = groups

	users, d := userAttributeValuesToSet(ctx, userAttributeUserValueObjectType, "user_uuid", attribute.Users)
	diags.Append(d...)
	model.Users = users

	return model, diags
}

func getUserAttributeResourceId(organizationUuid string, userAttributeUuid string) string {
	return fmt.Sprintf("organizations/%s/user-attributes/%s", organizationUuid, userAttributeUuid)
}

func extractUserAttributeResourceId(input string) (organizationUuid string, userAttributeUuid string, err error) {
	pattern := `^organizations/([^/]+)/user-attributes/([^/]+)$`
	groups, err := extractStrings(input, pattern)
	if err != nil {
		return "", "", fmt.Errorf("could not extract resource ID: %w", err)
	}
	return groups[0], groups[1], nil
}
