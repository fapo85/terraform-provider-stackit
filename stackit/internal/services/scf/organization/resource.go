package organization

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/validate"
	"net/http"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &scfOrganizationResource{}
	_ resource.ResourceWithConfigure   = &scfOrganizationResource{}
	_ resource.ResourceWithImportState = &scfOrganizationResource{}
)

type Model struct {
	Id         types.String `tfsdk:"id"` // Required by Terraform
	CreateAt   types.String `tfsdk:"created_at"`
	Name       types.String `tfsdk:"name"`
	PlatformId types.String `tfsdk:"platform_id"`
	ProjectId  types.String `tfsdk:"project_id"`
	QuotaId    types.String `tfsdk:"quota_id"`
	Region     types.String `tfsdk:"region"`
	Status     types.String `tfsdk:"status"`
	Suspended  types.Bool   `tfsdk:"suspended"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

// NewScfOrganizationResource is a helper function to create a new scf organization resource instance.
func NewScfOrganizationResource() resource.Resource {
	return &scfOrganizationResource{}
}

// scfOrganizationResource implements the resource interface for scf organization instances.
type scfOrganizationResource struct {
	client *scf.APIClient
}

// descriptions for the attributes in the Schema
var descriptions = map[string]string{
	//TODO id = org guid? Or with project and platform guid?
	"id":          "Terraform's internal resource ID, the globally unique identifier for the organization",
	"created_at":  "The time when the organization was created",
	"name":        "The name of the organization",
	"platform_id": "The ID of the platform associated with the organization",
	"project_id":  "The ID of the project associated with the organization",
	"quota_id":    "The ID of the quota associated with the organization",
	//TODO region from provider
	"region":     "The region where the organization is located",
	"status":     "The status of the organization (e.g., deleting, delete_failed)",
	"suspended":  "A boolean indicating whether the organization is suspended",
	"updated_at": "The time when the organization was last updated",
}

func (s scfOrganizationResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	providerData, ok := conversion.ParseProviderData(ctx, request.ProviderData, &response.Diagnostics)
	if !ok {
		return
	}

	apiClient := scfUtils.ConfigureClient(ctx, &providerData, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	s.client = apiClient
	tflog.Info(ctx, "scf client configured")
}

func (s scfOrganizationResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_scf_organization"
}

func (s scfOrganizationResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	//TODO how to map to a project, platform, region? Do we need a id out of multiple ids?
	orgGuid, err := uuid.Parse(request.ID)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics,
			"Error importing scf organization instance",
			fmt.Sprintf("Expected import identifier with UUID format. Got: %q", request.ID),
		)
		return
	}

	// Set the organization guid
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), orgGuid)...)
	tflog.Info(ctx, "Git instance state imported")
}

func (s scfOrganizationResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: descriptions["id"],
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: descriptions["created_at"],
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: descriptions["name"],
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"platform_id": schema.StringAttribute{
				Description: descriptions["platform_id"],
				Required:    false,
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
			"quota_id": schema.StringAttribute{
				Description: descriptions["quota_id"],
				Required:    false,
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"region": schema.StringAttribute{
				Description: descriptions["region"],
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: descriptions["status"],
				Computed:    true,
			},
			"suspended": schema.BoolAttribute{
				Description: descriptions["suspended"],
				Required:    false,
			},
			"updated_at": schema.StringAttribute{
				Description: descriptions["updated_at"],
				Computed:    true,
			},
		},
		Description: "STACKIT Cloud Foundry organization resource schema. Must have a `region` specified in the provider configuration.",
	}
}

func (s scfOrganizationResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	// Retrieve the planned values for the resource.
	var model Model
	diags := request.Plan.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Set logging context with the project ID and instance ID.
	projectId := model.ProjectId.ValueString()
	orgName := model.Name.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "org_name", orgName)

	payload, diags := toCreatePayload(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Create the new git instance via the API client.
	// TODO region
	scfOrgCreateResponse, err := s.client.CreateOrganization(ctx, projectId, "eu01").
		CreateOrganizationPayload(payload).
		Execute()
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error creating scf organization", fmt.Sprintf("Calling API to create org: %v", err))
		return
	}

	//TODO region
	scfOrgResponse, err := s.client.GetOrganization(ctx, projectId, "eu01", *scfOrgCreateResponse.Guid).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error creating scf organization", fmt.Sprintf("Calling API to load created org: %v", err))
		return
	}

	err = mapFields(ctx, scfOrgResponse, &model)
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

// Read refreshes the Terraform state with the latest scf organization data.
func (s scfOrganizationResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	// Retrieve the current state of the resource.
	var model Model
	diags := request.State.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Extract the project ID and instance id of the model
	projectId := model.ProjectId.ValueString()
	orgId := model.Id.ValueString()

	// Read the current scf organization via guid
	// TODO region
	scfOrgResponse, err := s.client.GetOrganization(ctx, projectId, "eu01", orgId).Execute()
	if err != nil {
		var oapiErr *oapierror.GenericOpenAPIError
		ok := errors.As(err, &oapiErr)
		if ok && oapiErr.StatusCode == http.StatusNotFound {
			response.State.RemoveResource(ctx)
			return
		}
		core.LogAndAddError(ctx, &response.Diagnostics, "Error reading scf organization", fmt.Sprintf("Calling API: %v", err))
		return
	}

	err = mapFields(ctx, scfOrgResponse, &model)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error reading scf organization", fmt.Sprintf("Processing API response: %v", err))
		return
	}

	// Set the updated state.
	diags = response.State.Set(ctx, &model)
	response.Diagnostics.Append(diags...)
	tflog.Info(ctx, fmt.Sprintf("read scf organization %s", orgId))
}

// Update attempts to update the resource.
func (s scfOrganizationResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	// Retrieve values from plan
	var model Model
	diags := request.Plan.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	projectId := model.ProjectId.ValueString()
	orgId := model.Id.ValueString()
	name := model.Name.ValueString()
	suspended := model.Suspended.ValueBool()

	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "org_id", orgId)

	// Retrieve values from state
	var stateModel Model
	diags = request.State.Get(ctx, &stateModel)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	//TODO will update be only called if there are changes or do we have to check?
	//if org, err := s.client.GetOrganization(ctx, model.ProjectId.ValueString(), model.Region.ValueString(), model.Id.ValueString()).Execute(); err != nil {
	//	core.LogAndAddError(ctx, &response.Diagnostics, "Error retrieving organization state", fmt.Sprintf("Getting organization state: %v", err))
	//}
	//if model.Name.ValueString() == *org.Name && model.Suspended.ValueBool() == *org.Suspended {

	updatedOrg, err := s.client.UpdateOrganization(ctx, projectId, model.Region.ValueString(), model.Id.ValueString()).UpdateOrganizationPayload(
		scf.UpdateOrganizationPayload{
			Name:      &name,
			Suspended: &suspended,
		}).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error updating organization", fmt.Sprintf("Processing API payload: %v", err))
		return
	}

	err = mapFields(ctx, updatedOrg, &model)
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error updating server", fmt.Sprintf("Processing API payload: %v", err))
		return
	}

	diags = response.State.Set(ctx, model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "organization updated")
}

// Delete deletes the git instance and removes it from the Terraform state on success.
func (s scfOrganizationResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	// Retrieve current state of the resource.
	var model Model
	diags := request.State.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()
	orgId := model.Id.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "org_id", orgId)

	// Call API to delete the existing scf organization.
	err, _ := s.client.DeleteOrganization(ctx, projectId, model.Region.ValueString(), orgId).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error deleting scf organization", fmt.Sprintf("Calling API: %v", err))
		return
	}

	//TODO wait for organization deletion how to get this into the SDK?

	tflog.Info(ctx, "Scf organization deleted")
}

// mapFields maps a SCF Organization response to the model.
func mapFields(ctx context.Context, response *scf.Organization, model *Model) error {
	if response == nil {
		return fmt.Errorf("response input is nil")
	}
	if model == nil {
		return fmt.Errorf("model input is nil")
	}

	if response.Guid == nil {
		return fmt.Errorf("SCF organization guid not present")
	}

	model.Id = types.StringPointerValue(response.Guid)
	model.CreateAt = types.StringValue(response.CreatedAt.String())
	model.Name = types.StringPointerValue(response.Name)
	model.PlatformId = types.StringPointerValue(response.PlatformId)
	model.ProjectId = types.StringPointerValue(response.ProjectId)
	model.QuotaId = types.StringPointerValue(response.QuotaId)
	model.Region = types.StringPointerValue(response.Region)
	model.Status = types.StringPointerValue(response.Status)
	model.Suspended = types.BoolPointerValue(response.Suspended)
	model.UpdatedAt = types.StringValue(response.UpdatedAt.String())

	return nil
}

// toCreatePayload creates the payload to create a scf organization instance
func toCreatePayload(ctx context.Context, model *Model) (scf.CreateOrganizationPayload, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	if model == nil {
		return scf.CreateOrganizationPayload{}, diags
	}

	payload := scf.CreateOrganizationPayload{
		Name: model.Name.ValueStringPointer(),
	}
	if !model.PlatformId.IsNull() {
		payload.PlatformId = model.PlatformId.ValueStringPointer()
	}
	return payload, diags
}
