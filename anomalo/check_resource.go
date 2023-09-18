package anomalo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/square/anomalo-go/anomalo"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &checkResource{}
	_ resource.ResourceWithConfigure   = &checkResource{}
	_ resource.ResourceWithImportState = &checkResource{}
)

func newCheckResource() resource.Resource {
	return &checkResource{}
}

type checkResource struct {
	client *anomalo.Client
}

// Values expected in the state & configuration
type checkResourceModel struct {
	TableID       types.Int64  `tfsdk:"table_id"`
	CheckType     types.String `tfsdk:"check_type"`
	CheckStaticID types.Int64  `tfsdk:"check_static_id"`
	Ref           types.String `tfsdk:"ref"`
	Params        types.Map    `tfsdk:"params"`
}

func (r *checkResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*anomalo.Client)
}

// Metadata returns the resource type name.
func (r *checkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

// Schema defines the schema for the resource.
func (r *checkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An Anomalo check. Closely maps to the check object in the Anomalo API. Updating system checks (checks with negative IDs) is not supported by the Anomalo API and thus is not supported by this resource.",
		Attributes: map[string]schema.Attribute{
			"check_static_id": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					// In an ideal world (with a fully featured anomalo API) we could use
					// int64planmodifier.UseStateForUnknown. However, we are unable to update checks in place & must
					// delete + recreate them.
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
				Description: "The check ID, persists through updates. Implementation Detail: The Anomalo API implements " +
					"check updates as a deletion of the old check + creation of a new one. When using this provider, " +
					"you can ignore that detail by using `static_check_id`. This makes the resource behave like a " +
					"typical HTTP resource.",
			},
			"table_id": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					// In an ideal world (with a fully featured anomalo API) we could use
					// int64planmodifier.UseStateForUnknown. However, we are unable to update checks in place & must
					// delete + recreate them.
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
				Description: "The ID of the table that this check belongs to. This can be specified by referencing " +
					"the resource object, ex `anomalo_table.<resource_name>.table_id`. It should not be changed after " +
					"creation.",
			},
			"check_type": schema.StringAttribute{
				Required: true,
				Description: "The type of check. Valid values are available in the Anomalo API documentation for " +
					"`create_check`.",
			},
			"ref": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Description: "A table-scoped, unique, human-readable identifier for the check that persists across " +
					"updates. This provider relies on check_static_id rather than ref changes to checks, so it's " +
					"possible to update the ref. If you used a version of this plugin before the attribute was " +
					"introduced, you may have specified check in the Params. The top level Ref (this attribute) will " +
					"take precedence if both are provided. Params-based refs may be unsupported in the future.",
			},
			"params": schema.MapAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "A map of parameters for the provided check type. Valid values are available in the " +
					"Anomalo API documentation for `create_check`. Acceptable values differ by check type.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *checkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	//Retrieve values from plan
	var plan checkResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.CheckStaticID.IsNull() && !plan.CheckStaticID.IsUnknown() {
		resp.Diagnostics.AddError(
			"Error Creating Check",
			fmt.Sprintf(
				"Specifying a `check_static_id` during creation is not supported. If you are "+
					"creating a new check, set check_static_id to null. If you are trying to update an existing "+
					"check, first import it with `terraform import <terraform-resource-identifier> %d,%d`",
				plan.TableID,
				plan.CheckStaticID,
			),
		)
		return
	}

	// Create the API request based on the plan
	var target map[string]string
	diags = plan.Params.ElementsAs(ctx, &target, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, ok := target["ref"]; ok {
		resp.Diagnostics.AddWarning("Ref defined in `params` for check",
			"Check with static ID %d for table ID %d had a `ref` defined in params. Ref is now a top level field"+
				"in the API, but may be present in `params` if you made a mistake or upgraded from a previous version of "+
				"this provider. The top level check ref, if defined, will take precedence. Param-based `ref`s may be "+
				"unsupported in future versions of the plugin. Removal of top level Ref will be ignored if one is "+
				"defined in the params.")
	}

	// Overwrite params-based ref if a top-level ref is provided.
	if !plan.Ref.IsUnknown() && !plan.Ref.IsNull() {
		target["ref"] = plan.Ref.ValueString()
	}

	createCheckReq := anomalo.CreateCheckRequest{
		TableID:   int(plan.TableID.ValueInt64()),
		CheckType: plan.CheckType.ValueString(),
		Params:    target,
	}

	// Create new check
	var createCheckResponse *anomalo.CreateCheckResponse
	createCheckResponse, err := r.client.CreateCheck(createCheckReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Check",
			fmt.Sprintf("Could not create check, unexpected error: %s", err.Error()),
		)
		return
	}

	// Map response body back into the plan, and set the state to plan values. Some values in the plan do not
	// change based on the API response, so we do not update them.
	plan.TableID = types.Int64Value(int64(createCheckReq.TableID))
	plan.CheckStaticID = types.Int64Value(int64(createCheckResponse.CheckStaticId))
	plan.Ref = types.StringValue(createCheckResponse.CheckRef)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *checkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state checkResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch the check from Anomalo
	var check *anomalo.Check
	var err error
	if int(state.CheckStaticID.ValueInt64()) != 0 {
		check, err = r.client.GetCheckByStaticID(int(state.TableID.ValueInt64()), int(state.CheckStaticID.ValueInt64()))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Checks",
				fmt.Sprintf("Could not read check for table ID %d, static ID %d, unexpected error: %s",
					state.TableID.ValueInt64(), state.CheckStaticID, err.Error()),
			)
			return
		}
	} else if state.Ref.ValueString() != "" {
		// Ref-only reads are supported only for imports. Unfortunately it's not easy to error when this isn't the case.
		resp.Diagnostics.AddWarning("Reading Check by Ref",
			fmt.Sprintf("The requested check has a static_id of 0. This should only happen when importing by "+
				"Ref. Table ID: %d, Ref: %s, StaticId: %d", state.TableID, state.Ref, state.CheckStaticID))
		check, err = r.client.GetCheckByRef(int(state.TableID.ValueInt64()), state.Ref.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Checks",
				fmt.Sprintf("Could not read check for table ID %d, ref %s, unexpected error: %s",
					state.TableID.ValueInt64(), state.Ref.ValueString(), err.Error()),
			)
			return
		}
	} else {
		// Not expected but possible if there is a bug elsewhere in the provider.
		resp.Diagnostics.AddError("Error Reading Check",
			fmt.Sprintf("Error in terraform state - a check for table ID %d has a static ID of 0 and Ref of %s. "+
				"Remove it from your state and re-import if necessary. If this issue persists notify the maintainer of "+
				"the provider.", state.TableID, state.Ref))
		return
	}

	if check == nil {
		// Check was deleted remotely
		resp.State.RemoveResource(ctx)
		return
	}

	// Convert map values into terraform types.
	mapVal := map[string]attr.Value{}
	for key, val := range check.Config.Params {
		if val != nil {
			// Note: consider using safer string conversion based on runtime type of val
			// There is no "generic" terraform type analogous to interface{}, so cast to a string
			mapVal[key] = types.StringValue(fmt.Sprintf("%v", val))
		}
	}
	mapParams, diag := types.MapValue(types.StringType, mapVal)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Map response body back into the state, and set the response state to the updated state.
	state.CheckStaticID = types.Int64Value(int64(check.CheckStaticID))
	state.CheckType = types.StringValue(check.Config.Check)
	state.Ref = types.StringValue(check.Ref)
	state.Params = mapParams

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *checkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan checkResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state checkResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.CheckStaticID.IsNull() && !plan.CheckStaticID.IsUnknown() && !plan.CheckStaticID.Equal(state.CheckStaticID) {
		resp.Diagnostics.AddError(
			"Error Updating Check",
			fmt.Sprintf("Updating the check_static_id to %s for table %s is not supported. If you want to create"+
				" a new check, delete this resource and create a new one with a null or undefined check_static_id. If "+
				"you are trying to manage an existing check, import it with "+
				"`terraform import <terraform-resource-identifier> %s,%s`",
				plan.CheckStaticID.String(), plan.TableID.String(), plan.TableID.String(), plan.CheckStaticID.String()),
		)
		return
	}

	// Make sure the check you're updating exists.
	existingCheck, err := r.client.GetCheckByStaticID(int(state.TableID.ValueInt64()), int(state.CheckStaticID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Check",
			fmt.Sprintf("Could not update check with static ID %d for table ID %d. Unable to find check to "+
				"delete. If you'd like to create this check, give it a new resource name and blank check_static_id. "+
				"Error: %s",
				state.CheckStaticID.ValueInt64(), state.TableID.ValueInt64(), err.Error()),
		)
		return
	}

	// "Updating" checks is (confusingly) accomplished by setting check_static_id in the Params of the Check we are
	// creating. Behind the scenes, Anomalo is creating a new check with a new ID and deleting the old check.

	// Build the request to Create a new check
	var target map[string]string
	diags = plan.Params.ElementsAs(ctx, &target, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	target["check_static_id"] = strconv.Itoa(existingCheck.CheckStaticID)

	if _, ok := target["ref"]; ok {
		resp.Diagnostics.AddWarning("Ref defined in `params` for check",
			"Check with static ID %d for table ID %d had a `ref` defined in params. Ref is now a top level field"+
				"in the API, but may be present in `params` if you made a mistake or upgraded from a previous version of "+
				"this provider. The top level check ref, if defined, will take precedence. Param-based `ref`s may be "+
				"unsupported in future versions of the plugin. Removal of top level Ref will be ignored if one is "+
				"defined in the params.")
	}

	// Overwrites param-based `Ref` if top level `Ref` exists.
	if !plan.Ref.IsUnknown() && !plan.Ref.IsNull() {
		target["ref"] = plan.Ref.ValueString()
	}

	checkType := plan.CheckType.ValueString()
	createCheckReq := anomalo.CreateCheckRequest{
		TableID:   int(plan.TableID.ValueInt64()),
		CheckType: checkType,
		Params:    target,
	}

	// Create new check
	createResponse, err := r.client.CreateCheck(createCheckReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Check",
			fmt.Sprintf("Could not create check with type %s for table %d, unexpected error: %s",
				createCheckReq.CheckType, createCheckReq.TableID, err.Error()),
		)
		return
	}

	// Most plan/state values should not change based on the response of CreateCheck.
	plan.CheckStaticID = types.Int64Value(int64(createResponse.CheckStaticId))
	plan.Ref = types.StringValue(createResponse.CheckRef) // A checkRef might be created if one does not exist.

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *checkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan checkResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existingCheck, err := r.client.GetCheckByStaticID(int(plan.TableID.ValueInt64()), int(plan.CheckStaticID.ValueInt64()))
	if err != nil || existingCheck == nil {
		resp.Diagnostics.AddError(
			"Error Deleting Check",
			fmt.Sprintf("Error deleting check with static ID %d on table ID %d. This may be due to a race "+
				"condition. If the error persists contact the provider maintainer. Unexpected error: %s",
				plan.CheckStaticID.ValueInt64(), plan.TableID.ValueInt64(), err.Error()),
		)
		return
	}

	deleteRequest := anomalo.DeleteCheckRequest{
		CheckID: existingCheck.CheckID,
		TableID: int(plan.TableID.ValueInt64()),
	}
	_, err = r.client.DeleteCheck(deleteRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Check",
			fmt.Sprintf("Could not delete check with ID %d for table ID %d, unexpected error: %s",
				plan.CheckStaticID.ValueInt64(), plan.TableID.ValueInt64(), err.Error()),
		)
		return
	}
}

func (r *checkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	tableID, err := strconv.Atoi(idParts[0])
	if err != nil {
		resp.Diagnostics.AddError("Error parsing table id from import identifier",
			fmt.Sprintf("Could not convert %s to int", idParts[0]))
	}

	if isStaticIdImport(idParts) {
		checkID, err := strconv.Atoi(idParts[1])
		if err != nil {
			resp.Diagnostics.AddError("Error parsing check id from import identifier",
				fmt.Sprintf("Could not convert %s to int", idParts[1]))
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table_id"), tableID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("check_static_id"), checkID)...)
	} else if isRefImport(idParts) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table_id"), tableID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ref"), idParts[2])...)
	} else {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: tableId,checkID or tableID,checkID,ref. Got: %q", req.ID),
		)
		return
	}
}

func isStaticIdImport(idParts []string) bool {
	properLength := len(idParts) == 2 || len(idParts) == 3
	hasTableIdAndCheckId := idParts[0] != "" && idParts[1] != ""
	doesNotHaveCheckRef := true
	if len(idParts) == 3 {
		doesNotHaveCheckRef = idParts[2] == ""
	}
	return properLength && hasTableIdAndCheckId && doesNotHaveCheckRef
}

func isRefImport(idParts []string) bool {
	properLength := len(idParts) == 3
	hasTableIdAndRef := idParts[0] != "" && idParts[2] != ""
	doesNotHaveCheckId := idParts[1] == ""
	return properLength && hasTableIdAndRef && doesNotHaveCheckId
}
