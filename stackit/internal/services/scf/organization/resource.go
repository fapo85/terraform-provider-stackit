package organization

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/utils"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/internal/validate"
	"net/http"
	"strings"
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
	OrgId      types.String `tfsdk:"org_id"`
	Region     types.String `tfsdk:"region"`
	Status     types.String `tfsdk:"status"`
	Suspended  types.Bool   `tfsdk:"suspended"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

// NewScfOrganizationResource is a helper function to create a new scf organization resource.
func NewScfOrganizationResource() resource.Resource {
	return &scfOrganizationResource{}
}

// scfOrganizationResource implements the resource interface for scf organization.
type scfOrganizationResource struct {
	client       *scf.APIClient
	providerData core.ProviderData
}

// descriptions for the attributes in the Schema
var descriptions = map[string]string{
	"id":          "Terraform's internal resource ID, structured as \"`project_id`,`org_id`\".",
	"created_at":  "The time when the organization was created",
	"name":        "The name of the organization",
	"platform_id": "The ID of the platform associated with the organization",
	"project_id":  "The ID of the project associated with the organization",
	"quota_id":    "The ID of the quota associated with the organization",
	"region":      "The region where the organization is located",
	"status":      "The status of the organization (e.g., deleting, delete_failed)",
	"suspended":   "A boolean indicating whether the organization is suspended",
	"org_id":      "The ID of the organization",
	"updated_at":  "The time when the organization was last updated",
}

func (s scfOrganizationResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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

func (s scfOrganizationResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_scf_organization"
}

func (s scfOrganizationResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	// Split the import identifier to extract project ID and email.
	idParts := strings.Split(request.ID, core.Separator)

	// Ensure the import identifier format is correct.
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		core.LogAndAddError(ctx, &response.Diagnostics,
			"Error importing scf organization",
			fmt.Sprintf("Expected import identifier with format: [project_id],[org_id]  Got: %q", request.ID),
		)
		return
	}

	projectId := idParts[0]
	orgGuid := idParts[1]
	// Set the project id and organization id in the state
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("project_id"), projectId)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("org_id"), orgGuid)...)
	tflog.Info(ctx, "Scf organization instance state imported")
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
			"org_id": schema.StringAttribute{
				Description: descriptions["org_id"],
				Computed:    true,
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

	// Create the new scf organization via the API client.
	scfOrgCreateResponse, err := s.client.CreateOrganization(ctx, projectId, s.providerData.GetRegion()).
		CreateOrganizationPayload(payload).
		Execute()
	if err != nil {
		core.LogAndAddError(ctx, &response.Diagnostics, "Error creating scf organization", fmt.Sprintf("Calling API to create org: %v", err))
		return
	}

	// Load the newly created scf organization
	scfOrgResponse, err := s.client.GetOrganization(ctx, projectId, s.providerData.GetRegion(), *scfOrgCreateResponse.Guid).Execute()
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
	orgId := model.OrgId.ValueString()

	// Read the current scf organization via guid
	scfOrgResponse, err := s.client.GetOrganization(ctx, projectId, s.providerData.GetRegion(), orgId).Execute()
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
	//TODO do we have to check if the region was changed and the throw an error as this is not supported?

	// Retrieve values from plan
	var model Model
	diags := request.Plan.Get(ctx, &model)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	projectId := model.ProjectId.ValueString()
	orgId := model.OrgId.ValueString()
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

	updatedOrg, err := s.client.UpdateOrganization(ctx, projectId, model.Region.ValueString(), model.OrgId.ValueString()).UpdateOrganizationPayload(
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
	orgId := model.OrgId.ValueString()
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

	// Build the ID by combining the project ID and organization id and assign the model's fields.
	model.Id = utils.BuildInternalTerraformId(model.ProjectId.ValueString(), *response.Guid)
	model.ProjectId = types.StringPointerValue(response.ProjectId)
	model.Region = types.StringPointerValue(response.Region)
	model.PlatformId = types.StringPointerValue(response.PlatformId)
	model.OrgId = types.StringPointerValue(response.Guid)
	model.Name = types.StringPointerValue(response.Name)
	model.Status = types.StringPointerValue(response.Status)
	model.Suspended = types.BoolPointerValue(response.Suspended)
	model.QuotaId = types.StringPointerValue(response.QuotaId)
	model.CreateAt = types.StringValue(response.CreatedAt.String())
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
