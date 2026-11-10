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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/api"
	apiv1 "github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/api/v1"
	"github.com/ubie-oss/terraform-provider-lightdash/internal/lightdash/models"
)

var ErrUserAttributeNotFound = errors.New("user attribute not found")

// ErrUserAttributeOrganizationMismatch means that the API token belongs to another organization
// than the user attribute, so the API can't tell whether the user attribute still exists.
var ErrUserAttributeOrganizationMismatch = errors.New("the API token belongs to another organization than the user attribute")

// UserAttributesService manages organization-level user attributes.
type UserAttributesService struct {
	client *api.Client
}

func NewUserAttributesService(client *api.Client) *UserAttributesService {
	return &UserAttributesService{client: client}
}

// List returns all user attributes in the organization of the API token.
func (s *UserAttributesService) List(ctx context.Context) ([]models.UserAttribute, error) {
	_ = ctx
	attributes, err := apiv1.GetUserAttributesV1(s.client)
	if err != nil {
		return nil, fmt.Errorf("failed to list user attributes: %w", err)
	}
	result := make([]models.UserAttribute, 0, len(attributes))
	for _, attribute := range attributes {
		result = append(result, toUserAttributeModel(attribute))
	}
	return result, nil
}

// GetByUuid returns the user attribute with the given UUID in the given organization.
// It returns ErrUserAttributeNotFound when the attribute does not exist,
// because the Lightdash API can only list user attributes.
//
// The list only has the attributes of the organization of the API token. A missing attribute
// is therefore not reported as not found when the token belongs to another organization,
// because the attribute may well exist there. ErrUserAttributeOrganizationMismatch is
// returned instead.
//
// The list is not cached like the organization members are, because every write of
// this provider changes it and Read would then report the values from before the write.
// The price is one list request per user attribute on every refresh.
func (s *UserAttributesService) GetByUuid(ctx context.Context, organizationUuid string, userAttributeUuid string) (*models.UserAttribute, error) {
	if strings.TrimSpace(userAttributeUuid) == "" {
		return nil, fmt.Errorf("user attribute UUID is empty")
	}

	attributes, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if attribute := findUserAttributeByUuid(attributes, userAttributeUuid); attribute != nil {
		if err := checkUserAttributeOrganization(organizationUuid, attribute.OrganizationUUID); err != nil {
			return nil, err
		}
		return attribute, nil
	}

	tokenOrganization, err := apiv1.GetMyOrganizationV1(s.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get the organization of the API token: %w", err)
	}
	if err := checkUserAttributeOrganization(organizationUuid, tokenOrganization.OrganizationUUID); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: UUID %q", ErrUserAttributeNotFound, userAttributeUuid)
}

// Create creates a user attribute. The UUIDs in the input are ignored.
func (s *UserAttributesService) Create(ctx context.Context, attribute models.UserAttribute) (*models.UserAttribute, error) {
	_ = ctx
	created, err := apiv1.CreateUserAttributeV1(s.client, toUserAttributeV1Request(attribute))
	if err != nil {
		return nil, fmt.Errorf("failed to create user attribute: %w", err)
	}
	result := toUserAttributeModel(*created)
	return &result, nil
}

// Update replaces the user attribute, including all of its user and group values.
func (s *UserAttributesService) Update(ctx context.Context, attribute models.UserAttribute) (*models.UserAttribute, error) {
	_ = ctx
	updated, err := apiv1.UpdateUserAttributeV1(s.client, attribute.UserAttributeUUID, toUserAttributeV1Request(attribute))
	if err != nil {
		return nil, fmt.Errorf("failed to update user attribute: %w", err)
	}
	result := toUserAttributeModel(*updated)
	return &result, nil
}

// Delete deletes the user attribute. It succeeds when the attribute is already gone.
// The error of the API is not inspected for that, because the API answers a missing
// attribute with the same generic 500 as any other failure. The list is checked instead,
// and only a list of the attribute's own organization counts as proof that it is gone.
func (s *UserAttributesService) Delete(ctx context.Context, organizationUuid string, userAttributeUuid string) error {
	deleteErr := apiv1.DeleteUserAttributeV1(s.client, userAttributeUuid)
	if deleteErr == nil {
		return nil
	}
	if _, getErr := s.GetByUuid(ctx, organizationUuid, userAttributeUuid); errors.Is(getErr, ErrUserAttributeNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete user attribute: %w", deleteErr)
}

// findUserAttributeByUuid returns nil when no attribute has the UUID.
func findUserAttributeByUuid(attributes []models.UserAttribute, userAttributeUuid string) *models.UserAttribute {
	for i := range attributes {
		if attributes[i].UserAttributeUUID == userAttributeUuid {
			return &attributes[i]
		}
	}
	return nil
}

func checkUserAttributeOrganization(expectedOrganizationUuid string, actualOrganizationUuid string) error {
	if expectedOrganizationUuid != actualOrganizationUuid {
		return fmt.Errorf("%w: expected organization %q, got %q", ErrUserAttributeOrganizationMismatch, expectedOrganizationUuid, actualOrganizationUuid)
	}
	return nil
}

func toUserAttributeModel(attribute apiv1.UserAttributeV1) models.UserAttribute {
	description := ""
	if attribute.Description != nil {
		description = *attribute.Description
	}

	users := make([]models.UserAttributeValue, 0, len(attribute.Users))
	for _, user := range attribute.Users {
		users = append(users, models.UserAttributeValue{UUID: user.UserUUID, Values: user.Values})
	}

	groups := make([]models.UserAttributeValue, 0, len(attribute.Groups))
	for _, group := range attribute.Groups {
		groups = append(groups, models.UserAttributeValue{UUID: group.GroupUUID, Values: group.Values})
	}

	return models.UserAttribute{
		OrganizationUUID:  attribute.OrganizationUUID,
		UserAttributeUUID: attribute.UUID,
		Name:              attribute.Name,
		Description:       description,
		AttributeDefaults: attribute.AttributeDefaults,
		Users:             users,
		Groups:            groups,
	}
}

func toUserAttributeV1Request(attribute models.UserAttribute) apiv1.UserAttributeV1Request {
	users := make([]apiv1.UserAttributeV1RequestUserValue, 0, len(attribute.Users))
	for _, user := range attribute.Users {
		users = append(users, apiv1.UserAttributeV1RequestUserValue{UserUUID: user.UUID, Values: user.Values})
	}

	groups := make([]apiv1.UserAttributeV1RequestGroupValue, 0, len(attribute.Groups))
	for _, group := range attribute.Groups {
		groups = append(groups, apiv1.UserAttributeV1RequestGroupValue{GroupUUID: group.UUID, Values: group.Values})
	}

	return apiv1.UserAttributeV1Request{
		Name:              attribute.Name,
		Description:       attribute.Description,
		AttributeDefaults: attribute.AttributeDefaults,
		Users:             users,
		Groups:            groups,
	}
}
