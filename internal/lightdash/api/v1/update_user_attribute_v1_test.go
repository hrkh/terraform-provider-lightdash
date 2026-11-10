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

import "testing"

func TestUpdateUserAttributeV1_validation(t *testing.T) {
	if _, err := UpdateUserAttributeV1(nil, " ", UserAttributeV1Request{Name: "region"}); err == nil {
		t.Errorf("expected an error for an empty UUID")
	}
	if _, err := UpdateUserAttributeV1(nil, "d47a8687-8bc7-4bc4-950d-0031fbfb6e22", UserAttributeV1Request{}); err == nil {
		t.Errorf("expected an error for an empty name")
	}
}
