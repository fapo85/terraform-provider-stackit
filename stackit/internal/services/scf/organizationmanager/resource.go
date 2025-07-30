package organizationmanager

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stackitcloud/stackit-sdk-go/core/oapierror"
	"github.com/stackitcloud/stackit-sdk-go/services/scf"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/conversion"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/core"
	scfUtils "github.com/stackitcloud/terraform-provider-stackit/stackit/internal/services/scf/utils"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/utils"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/validate"
	"net/http"
	"strings"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &scfOrganizationManagerResource{}
	_ resource.ResourceWithConfigure   = &scfOrganizationManagerResource{}
	_ resource.ResourceWithImportState = &scfOrganizationManagerResource{}
)

type Model struct {
	Id         types.String `tfsdk:"id"` // Required by Terraform
	Region     types.String `tfsdk:"region"`
	PlatformId types.String `tfsdk:"platform_id"`
	ProjectId  types.String `tfsdk:"project_id"`
	OrgId      types.String `tfsdk:"org_id"`
	UserId     types.String `tfsdk:"user_id"`
	UserName   types.String `tfsdk:"username"`
	Password   types.String `tfsdk:"password"`
	CreateAt   types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

// NewScfOrganizationManagerResource is a helper function to create a new scf organization manager resource.
func NewScfOrganizationManagerResource() resource.Resource {
	return &scfOrganizationManagerResource{}
}

// scfOrganizationManagerResource implements the resource interface for scf organization manager.
type scfOrganizationManagerResource struct {
	client       *scf.APIClient
	providerData core.ProviderData
}

// descriptions for the attributes in the Schema
var descriptions = map[string]string{
	"id":          "Terraform's internal resource ID, structured as \"`project_id`,`user_id`\".",
	"region":      "The region where the organization of the organization manager is located",
	"platform_id": "The ID of the platform associated with the organization of the organization manager",
	"project_id":  "The ID of the project associated with the organization of the organization manager",
	"org_id":      "The ID of the organization",
	"user_id":     "The ID of the organization manager user",
	"username":    "An auto-generated organization manager user name",
	"password":    "An auto-generated password",
	"created_at":  "The time when the organization manager was created",
	"updated_at":  "The time when the organization manager was last updated",
}

func (s scfOrganizationManagerResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	var ok bool
	s.providerData, ok = conversion.ParseProviderData(ctx, request.ProviderData, &response.Diagnostics)
	if !ok {
		return
	}

	apiClient := scfUtils.ConfigureClient(ctx, &s.providerData, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	s.client = apiClient
	tflog.Info(ctx, "scf client configured")
}

func (s scfOrganizationManagerResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_scf_organization_manager"
}

func (s scfOrganizationManagerResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	// Split the import identifier to extract project ID and email.
	idParts := strings.Split(request.ID, core.Separator)

	// Ensure the import identifier format is correct.
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		core.LogAndAddError(ctx, &response.Diagnostics,
			"Error importing scf organization manager",
			fmt.Sprintf("Expected import identifier with format: [project_id],[user_id]  Got: %q", request.ID),
		)
		return
	}

	projectId := idParts[0]
	userId := idParts[1]
	// Set the project id and organization id in the state
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("project_id"), projectId)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("user_id"), userId)...)
	tflog.Info(ctx, "Scf organization manager state imported")
}

func (s scfOrganizationManagerResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: descriptions["id"],
				Computed:    true,
			},
			"region": schema.StringAttribute{
				Description: descriptions["region"],
				Computed:    true,
			},
			"platform_id": schema.StringAttribute{
				Description: descriptions["platform_id"],
				Computed:    true,
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: descriptions["project_id"],
				Required:    true,
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"org_id": schema.StringAttribute{
				Description: descriptions["org_id"],
				Required:    true,
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"user_id": schema.StringAttribute{
				Description: descriptions["user_id"],
				Computed:    true,
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"username": schema.StringAttribute{
				Description: descriptions["username"],
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"password": schema.StringAttribute{
				Description: descriptions["password"],
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"created_at": schema.StringAttribute{
				Description: descriptions["created_at"],
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: descriptions["updated_at"],
				Computed:    true,
			},
		},
		Description: "STACKIT Cloud Foundry organization manager resource schema. Must have a `region` specified in the provider configuration.",
	}
}

func (s scfOrganizationManagerResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	// Retrieve the planned values for the resource.
	var model Model
	diags := request.Plan.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Set logging context with the project ID and instance ID.
	projectId := model.ProjectId.ValueString()
	orgId := model.OrgId.ValueString()
	userName := model.UserName.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "username", userName)

	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Create the new scf organization manager via the API client.
	scfOrgManagerCreateResponse, err := s.client.CreateOrgManagerExecute(ctx, projectId, s.providerData.GetRegion(), orgId)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error creating scf organization manager", fmt.Sprintf("Calling API to create org manager: %v", err))
		return
	}

	err = mapFieldsCreate(scfOrgManagerCreateResponse, &model)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error creating scf organization", fmt.Sprintf("Mapping fields: %v", err))
		return
	}

	// Set the state with fully populated data.
	diags = response.State.Set(ctx, model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "Scf organization created")
}

func (s scfOrganizationManagerResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	// Retrieve the current state of the resource.
	var model Model
	diags := request.State.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Extract the project ID and instance id of the model
	projectId := model.ProjectId.ValueString()
	orgId := model.OrgId.ValueString()

	// Read the current scf organization manager via orgId
	scfOrgManager, err := s.client.GetOrgManagerExecute(ctx, projectId, s.providerData.GetRegion(), orgId)
	if err != nil {
		var oapiErr *oapierror.GenericOpenAPIError
		ok := errors.As(err, &oapiErr)
		if ok && oapiErr.StatusCode == http.StatusNotFound {
			response.State.RemoveResource(ctx)
			return
		}
		core.LogAndAddError(ctx, &response.Diagnostics, "Error reading scf organization manager", fmt.Sprintf("Calling API: %v", err))
		return
	}

	err = mapFieldsUpdate(scfOrgManager, &model)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error reading scf organization manager", fmt.Sprintf("Processing API response: %v", err))
		return
	}

	// Set the updated state.
	diags = response.State.Set(ctx, &model)
	response.Diagnostics.Append(diags...)
	tflog.Info(ctx, fmt.Sprintf("read scf organization %s", orgId))
}

func (s scfOrganizationManagerResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	// organization manager cannot be updated, so we log an error.
	core.LogAndAddError(ctx, &response.Diagnostics, "Error updating organization manager", "Organization Manager can't be updated")
}

func (s scfOrganizationManagerResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	// Retrieve current state of the resource.
	var model Model
	diags := request.State.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()
	orgId := model.OrgId.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "org_id", orgId)

	// Call API to delete the existing scf organization.
	err, _ := s.client.DeleteOrgManagerExecute(ctx, projectId, model.Region.ValueString(), orgId)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error deleting scf organization manager", fmt.Sprintf("Calling API: %v", err))
		return
	}
	tflog.Info(ctx, "Scf organization deleted")
}

func mapFieldsCreate(response *scf.OrgManagerResponse, model *Model) error {
	if response == nil {
		return fmt.Errorf("response input is nil")
	}
	if model == nil {
		return fmt.Errorf("model input is nil")
	}

	if response.Guid == nil {
		return fmt.Errorf("SCF organization manager user_id not present")
	}

	// Build the ID by combining the project ID and organization id and assign the model's fields.
	model.Id = utils.BuildInternalTerraformId(model.ProjectId.ValueString(), *response.Guid)
	model.Region = types.StringPointerValue(response.Region)
	model.PlatformId = types.StringPointerValue(response.PlatformId)
	model.ProjectId = types.StringPointerValue(response.ProjectId)
	model.OrgId = types.StringPointerValue(response.OrgId)
	model.UserId = types.StringPointerValue(response.Guid)
	model.UserName = types.StringPointerValue(response.Username)
	model.Password = types.StringPointerValue(response.Password)
	model.CreateAt = types.StringValue(response.CreatedAt.String())
	model.UpdatedAt = types.StringValue(response.UpdatedAt.String())
	return nil
}

func mapFieldsUpdate(response *scf.OrgManager, model *Model) error {
	if response == nil {
		return fmt.Errorf("response input is nil")
	}
	if model == nil {
		return fmt.Errorf("model input is nil")
	}

	if response.Guid == nil {
		return fmt.Errorf("SCF organization manager user_id not present")
	}

	// Build the ID by combining the project ID and organization id and assign the model's fields.
	model.Id = utils.BuildInternalTerraformId(model.ProjectId.ValueString(), *response.Guid)
	model.Region = types.StringPointerValue(response.Region)
	model.PlatformId = types.StringPointerValue(response.PlatformId)
	model.ProjectId = types.StringPointerValue(response.ProjectId)
	model.UserId = types.StringPointerValue(response.Guid)
	model.UserName = types.StringPointerValue(response.Username)
	model.CreateAt = types.StringValue(response.CreatedAt.String())
	model.UpdatedAt = types.StringValue(response.UpdatedAt.String())
	return nil
}
