package organization

import (
	"fmt"
	"github.com/stackitcloud/stackit-sdk-go/services/scf"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackitcloud/stackit-sdk-go/core/utils"
)

var (
	testOrgId      = uuid.New().String()
	testProjectId  = uuid.New().String()
	testPlatformId = uuid.New().String()
	testQuotaId    = uuid.New().String()
)

func TestMapFields(t *testing.T) {
	createdTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-01-01 00:00:00 +0000 UTC")
	if err != nil {
		t.Fatalf("failed to parse test time: %v", err)
	}

	tests := []struct {
		description string
		input       *scf.Organization
		expected    *Model
		isValid     bool
	}{
		{
			description: "minimal_input",
			input: &scf.Organization{
				Guid: utils.Ptr(testOrgId),
				Name: utils.Ptr("scf-org-min-instance"),
			},
			expected: &Model{
				Id:         types.StringValue(fmt.Sprintf("%s,%s", testProjectId, testOrgId)),
				ProjectId:  types.StringValue(testProjectId),
				Region:     types.StringNull(),
				Name:       types.StringValue("scf-org-min-instance"),
				PlatformId: types.StringNull(),
				OrgId:      types.StringNull(),
				CreateAt:   types.StringNull(),
				QuotaId:    types.StringNull(),
				Status:     types.StringNull(),
				Suspended:  types.BoolNull(),
				UpdatedAt:  types.StringNull(),
			},
			isValid: true,
		},
		{
			description: "max_input",
			input: &scf.Organization{
				CreatedAt:  &createdTime,
				Guid:       utils.Ptr(testOrgId),
				Name:       utils.Ptr("scf-full-org"),
				PlatformId: utils.Ptr(testPlatformId),
				ProjectId:  utils.Ptr(testProjectId),
				QuotaId:    utils.Ptr(testQuotaId),
				Region:     utils.Ptr(testutil.Region),
				Status:     utils.Ptr(""),
				Suspended:  utils.Ptr(true),
				UpdatedAt:  &createdTime,
			},
			expected: &Model{
				Id:         types.StringValue(fmt.Sprintf("%s,%s", testProjectId, testOrgId)),
				ProjectId:  types.StringValue(testProjectId),
				OrgId:      types.StringValue(testOrgId),
				Name:       types.StringValue("scf-full-instance"),
				Region:     types.StringValue(testutil.Region),
				PlatformId: types.StringValue(testPlatformId),
				CreateAt:   types.StringValue("2025-01-01 00:00:00 +0000 UTC"),
				UpdatedAt:  types.StringValue("2025-01-01 00:00:00 +0000 UTC"),
				QuotaId:    types.StringValue(testQuotaId),
				Status:     types.StringNull(),
				Suspended:  types.BoolValue(true),
			},
			isValid: true,
		},
		{
			description: "nil_org",
			input:       nil,
			expected:    nil,
			isValid:     false,
		},
		{
			description: "empty_org",
			input:       &scf.Organization{},
			expected:    nil,
			isValid:     false,
		},
		{
			description: "missing_id",
			input: &scf.Organization{
				Name: utils.Ptr("scf-missing-id"),
			},
			expected: nil,
			isValid:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			state := &Model{}
			if tt.expected != nil {
				state.ProjectId = tt.expected.ProjectId
			}
			err := mapFields(tt.input, state)

			if tt.isValid && err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}
			if !tt.isValid && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tt.isValid {
				if diff := cmp.Diff(tt.expected, state); diff != "" {
					t.Errorf("unexpected diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestToCreatePayload(t *testing.T) {
	tests := []struct {
		description string
		input       *Model
		expected    scf.CreateOrganizationPayload
		expectError bool
	}{
		{
			description: "default values",
			input: &Model{
				Name:       types.StringValue("example-org"),
				PlatformId: types.StringValue(testPlatformId),
			},
			expected: scf.CreateOrganizationPayload{
				Name:       utils.Ptr("example-org"),
				PlatformId: utils.Ptr(testPlatformId),
			},
			expectError: false,
		},
		{
			description: "nil input model",
			input:       nil,
			expected:    scf.CreateOrganizationPayload{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			output, diags := toCreatePayload(tt.input)

			if tt.expectError && !diags.HasError() {
				t.Fatalf("expected diagnostics error but got none")
			}

			if !tt.expectError && diags.HasError() {
				t.Fatalf("unexpected diagnostics error: %v", diags)
			}

			if diff := cmp.Diff(tt.expected, output); diff != "" {
				t.Fatalf("unexpected payload (-want +got):\n%s", diff)
			}
		})
	}
}
