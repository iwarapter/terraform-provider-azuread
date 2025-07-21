// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package synchronization

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/microsoft-graph/common-types/stable"
	"github.com/hashicorp/go-azure-sdk/microsoft-graph/serviceprincipals/stable/synchronizationjobschema"
	"github.com/hashicorp/go-azure-sdk/sdk/nullable"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
)

type SynchronizationJobSchemaResource struct {
	client *clients.Client
}

type SynchronizationJobSchemaResourceModel struct {
	Timeouts             timeouts.Value `tfsdk:"timeouts"`
	SynchronizationJobId types.String   `tfsdk:"synchronization_job_id"`
	SynchronizationRules types.List     `tfsdk:"synchronization_rule"`
	Id                   types.String   `tfsdk:"id"`
}

type SynchronizationRuleModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	SourceDirectoryName types.String `tfsdk:"source_directory_name"`
	TargetDirectoryName types.String `tfsdk:"target_directory_name"`
	ObjectMappings      types.List   `tfsdk:"object_mapping"`
}

type ObjectMappingModel struct {
	Enabled          types.Bool   `tfsdk:"enabled"`
	FlowTypes        types.String `tfsdk:"flow_types"`
	Name             types.String `tfsdk:"name"`
	SourceObjectName types.String `tfsdk:"source_object_name"`
	TargetObjectName types.String `tfsdk:"target_object_name"`
	Attributes       types.List   `tfsdk:"attribute"`
}

type AttributeModel struct {
	DefaultValue            types.String          `tfsdk:"default_value"`
	ExportMissingReferences types.Bool            `tfsdk:"export_missing_references"`
	FlowBehavior            types.String          `tfsdk:"flow_behavior"`
	FlowType                types.String          `tfsdk:"flow_type"`
	MatchingPriority        types.Int64           `tfsdk:"matching_priority"`
	TargetAttributeName     types.String          `tfsdk:"target_attribute_name"`
	Source                  *AttributeSourceModel `tfsdk:"source"`
}

type AttributeSourceModel struct {
	Expression types.String `tfsdk:"expression"`
	Name       types.String `tfsdk:"name"`
	Parameters types.List   `tfsdk:"parameters"`
	Type       types.String `tfsdk:"type"`
}

type AttributeSourceParameterModel struct {
	Key   types.String             `tfsdk:"key"`
	Value *AttributeParameterModel `tfsdk:"value"`
}

type AttributeParameterModel struct {
	Expression types.String `tfsdk:"expression"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
}

var (
	_ resource.Resource                = &SynchronizationJobSchemaResource{}
	_ resource.ResourceWithConfigure   = &SynchronizationJobSchemaResource{}
	_ resource.ResourceWithImportState = &SynchronizationJobSchemaResource{}
)

func NewSynchronizationJobSchemaResource() resource.Resource {
	return &SynchronizationJobSchemaResource{}
}

func (r *SynchronizationJobSchemaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*clients.Client)
}

func (r *SynchronizationJobSchemaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_synchronization_job_schema"
}

func (r *SynchronizationJobSchemaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a synchronization job schema for an Azure AD service principal.",

		Attributes: map[string]schema.Attribute{
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Read:   true,
				Create: true,
				Update: true,
				Delete: true,
			}),
			"synchronization_job_id": schema.StringAttribute{
				Description: "The ID of the synchronization job for which this schema should be created",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"synchronization_rule": schema.ListNestedAttribute{
				Description: "The synchronization rules for this schema",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "The ID of the synchronization rule",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"name": schema.StringAttribute{
							Description: "The name of the synchronization rule",
							Required:    true,
						},
						"source_directory_name": schema.StringAttribute{
							Description: "The name of the source directory",
							Required:    true,
						},
						"target_directory_name": schema.StringAttribute{
							Description: "The name of the target directory",
							Required:    true,
						},
						"object_mapping": schema.ListNestedAttribute{
							Description: "The object mappings for this rule",
							Optional:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"enabled": schema.BoolAttribute{
										Description: "Whether this object mapping is enabled",
										Required:    true,
									},
									"flow_types": schema.StringAttribute{
										Description: "The flow types for this object mapping",
										Optional:    true,
									},
									"name": schema.StringAttribute{
										Description: "The name of the object mapping",
										Required:    true,
									},
									"source_object_name": schema.StringAttribute{
										Description: "The name of the source object",
										Required:    true,
									},
									"target_object_name": schema.StringAttribute{
										Description: "The name of the target object",
										Required:    true,
									},
									"attribute": schema.ListNestedAttribute{
										Description: "The attributes for this object mapping",
										Optional:    true,
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"default_value": schema.StringAttribute{
													Description: "The default value for this attribute",
													Optional:    true,
												},
												"export_missing_references": schema.BoolAttribute{
													Description: "Whether to export missing references",
													Optional:    true,
												},
												"flow_behavior": schema.StringAttribute{
													Description: "The flow behavior for this attribute",
													Optional:    true,
												},
												"flow_type": schema.StringAttribute{
													Description: "The flow type for this attribute",
													Optional:    true,
												},
												"matching_priority": schema.Int64Attribute{
													Description: "The matching priority for this attribute",
													Optional:    true,
												},
												"target_attribute_name": schema.StringAttribute{
													Description: "The name of the target attribute",
													Required:    true,
												},
												"source": schema.SingleNestedAttribute{
													Description: "The source configuration for this attribute",
													Optional:    true,
													Attributes: map[string]schema.Attribute{
														"expression": schema.StringAttribute{
															Description: "The expression for this source",
															Optional:    true,
														},
														"name": schema.StringAttribute{
															Description: "The name of this source",
															Optional:    true,
														},
														"parameters": schema.ListNestedAttribute{
															Description: "The parameters for this source",
															Optional:    true,
															NestedObject: schema.NestedAttributeObject{
																Attributes: map[string]schema.Attribute{
																	"key": schema.StringAttribute{
																		Description: "The key for this parameter",
																		Required:    true,
																	},
																	"value": schema.SingleNestedAttribute{
																		Description: "The source configuration for this attribute",
																		Optional:    true,
																		Attributes: map[string]schema.Attribute{
																			"expression": schema.StringAttribute{
																				Description: "The expression for this source",
																				Optional:    true,
																			},
																			"name": schema.StringAttribute{
																				Description: "The name of this source",
																				Optional:    true,
																			},
																			"type": schema.StringAttribute{
																				Description: "The type of this source",
																				Optional:    true,
																			},
																		},
																	},
																},
															},
														},
														"type": schema.StringAttribute{
															Description: "The type of this source",
															Optional:    true,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"id": schema.StringAttribute{
				Description: "The ID of the synchronization job schema",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *SynchronizationJobSchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("synchronization_job_id"), req.ID)...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SynchronizationJobSchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SynchronizationJobSchemaResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	client := r.client.Synchronization.SynchronizationJobSchemaClient

	synchronizationJobId, err := stable.ParseServicePrincipalIdSynchronizationJobID(data.SynchronizationJobId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid synchronization job ID", fmt.Sprintf("Unable to parse synchronization job ID: %s", err))
		return
	}

	schemaData := expandSynchronizationJobSchema(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	getSchema, err := client.GetSynchronizationJobSchema(ctx, *synchronizationJobId, synchronizationjobschema.DefaultGetSynchronizationJobSchemaOperationOptions())
	if err != nil {
		resp.Diagnostics.AddError("Error reading synchronization job schema", fmt.Sprintf("Unable to read synchronization job schema: %s", err))
		return
	}

	schemaData.Directories = getSchema.Model.Directories
	setMetadata(getSchema, schemaData)

	_, err = client.UpdateSynchronizationJobSchema(ctx, *synchronizationJobId, schemaData, synchronizationjobschema.DefaultUpdateSynchronizationJobSchemaOperationOptions())
	if err != nil {
		resp.Diagnostics.AddError("Error creating synchronization job schema", fmt.Sprintf("Unable to create synchronization job schema: %s", err))
		return
	}

	data.Id = types.StringValue(synchronizationJobId.ID())
	data.SynchronizationRules = flattenSynchronizationRules(schemaData.SynchronizationRules)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

// sets the metadata for each rule and object mapping if not provided
func setMetadata(getSchema synchronizationjobschema.GetSynchronizationJobSchemaOperationResponse, schemaData stable.SynchronizationSchema) {
	for i, responseRule := range *getSchema.Model.SynchronizationRules {
		for _, rule := range *schemaData.SynchronizationRules {
			if !rule.Name.IsNull() && !responseRule.Name.IsNull() {
				if rule.Name.GetOrZero() == responseRule.Name.GetOrZero() {
					if (*schemaData.SynchronizationRules)[i].Id.IsNull() {
						(*schemaData.SynchronizationRules)[i].Id = responseRule.Id
					}
					if (*schemaData.SynchronizationRules)[i].Metadata == nil {
						(*schemaData.SynchronizationRules)[i].Metadata = responseRule.Metadata
					}
					for idx, mapping := range *(*schemaData.SynchronizationRules)[i].ObjectMappings {
						for _, dataMapping := range *rule.ObjectMappings {
							if !mapping.Name.IsNull() && !dataMapping.Name.IsNull() && mapping.Name.GetOrZero() == dataMapping.Name.GetOrZero() {
								if (*(*schemaData.SynchronizationRules)[i].ObjectMappings)[idx].Metadata == nil {
									(*(*schemaData.SynchronizationRules)[i].ObjectMappings)[idx].Metadata = dataMapping.Metadata
								}
							}
						}
					}
				}
			}
		}
	}
}

func (r *SynchronizationJobSchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SynchronizationJobSchemaResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 5*time.Minute)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	client := r.client.Synchronization.SynchronizationJobSchemaClient

	synchronizationJobId, err := stable.ParseServicePrincipalIdSynchronizationJobID(data.SynchronizationJobId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid synchronization job ID", fmt.Sprintf("Unable to parse synchronization job ID: %s", err))
		return
	}

	schemaResp, err := client.GetSynchronizationJobSchema(ctx, *synchronizationJobId, synchronizationjobschema.DefaultGetSynchronizationJobSchemaOperationOptions())
	if err != nil {
		if response.WasNotFound(schemaResp.HttpResponse) {
			log.Printf("[DEBUG] %s was not found - removing from state!", synchronizationJobId)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error retrieving synchronization job schema", fmt.Sprintf("Unable to retrieve synchronization job schema: %s", err))
		return
	}

	if schemaResp.Model == nil {
		resp.Diagnostics.AddError("Error retrieving synchronization job schema", "Model was nil")
		return
	}

	data.Id = types.StringValue(synchronizationJobId.ID())
	data.SynchronizationJobId = types.StringValue(synchronizationJobId.ID())
	data.SynchronizationRules = flattenSynchronizationRules(schemaResp.Model.SynchronizationRules)

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *SynchronizationJobSchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SynchronizationJobSchemaResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	client := r.client.Synchronization.SynchronizationJobSchemaClient

	synchronizationJobId, err := stable.ParseServicePrincipalIdSynchronizationJobID(data.SynchronizationJobId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid synchronization job ID", fmt.Sprintf("Unable to parse synchronization job ID: %s", err))
		return
	}

	schemaData := expandSynchronizationJobSchema(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	getSchema, err := client.GetSynchronizationJobSchema(ctx, *synchronizationJobId, synchronizationjobschema.DefaultGetSynchronizationJobSchemaOperationOptions())
	if err != nil {
		resp.Diagnostics.AddError("Error reading synchronization job schema", fmt.Sprintf("Unable to read synchronization job schema: %s", err))
		return
	}

	schemaData.Directories = getSchema.Model.Directories
	setMetadata(getSchema, schemaData)

	_, err = client.UpdateSynchronizationJobSchema(ctx, *synchronizationJobId, schemaData, synchronizationjobschema.DefaultUpdateSynchronizationJobSchemaOperationOptions())
	if err != nil {
		resp.Diagnostics.AddError("Error updating synchronization job schema", fmt.Sprintf("Unable to update synchronization job schema: %s", err))
		return
	}
	data.SynchronizationRules = flattenSynchronizationRules(schemaData.SynchronizationRules)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *SynchronizationJobSchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SynchronizationJobSchemaResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := data.Timeouts.Delete(ctx, 5*time.Minute)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	client := r.client.Synchronization.SynchronizationJobSchemaClient

	synchronizationJobId, err := stable.ParseServicePrincipalIdSynchronizationJobID(data.SynchronizationJobId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid synchronization job ID", fmt.Sprintf("Unable to parse synchronization job ID: %s", err))
		return
	}

	_, err = client.DeleteSynchronizationJobSchema(ctx, *synchronizationJobId, synchronizationjobschema.DefaultDeleteSynchronizationJobSchemaOperationOptions())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting synchronization job schema", fmt.Sprintf("Unable to delete synchronization job schema: %s", err))
		return
	}
}

func expandSynchronizationJobSchema(ctx context.Context, data SynchronizationJobSchemaResourceModel, diags *diag.Diagnostics) stable.SynchronizationSchema {
	return stable.SynchronizationSchema{
		SynchronizationRules: expandSynchronizationRules(ctx, data, diags),
	}
}

func expandSynchronizationRules(ctx context.Context, data SynchronizationJobSchemaResourceModel, diags *diag.Diagnostics) *[]stable.SynchronizationRule {
	if data.SynchronizationRules.IsNull() || data.SynchronizationRules.IsUnknown() {
		return nil
	}

	var rules []SynchronizationRuleModel
	diags.Append(data.SynchronizationRules.ElementsAs(ctx, &rules, false)...)
	if diags.HasError() {
		return nil
	}

	expandedRules := make([]stable.SynchronizationRule, 0)

	for _, rule := range rules {
		expandedRule := stable.SynchronizationRule{
			Name:                nullable.Value(rule.Name.ValueString()),
			SourceDirectoryName: nullable.Value(rule.SourceDirectoryName.ValueString()),
			TargetDirectoryName: nullable.Value(rule.TargetDirectoryName.ValueString()),
		}

		if !rule.Id.IsNull() && !rule.Id.IsUnknown() {
			expandedRule.Id = nullable.Value(rule.Id.ValueString())
		}

		if !rule.ObjectMappings.IsNull() && !rule.ObjectMappings.IsUnknown() {
			expandedRule.ObjectMappings = expandObjectMappings(ctx, rule.ObjectMappings, diags)
		}

		expandedRules = append(expandedRules, expandedRule)
	}

	return &expandedRules
}

func expandObjectMappings(ctx context.Context, mappings types.List, diags *diag.Diagnostics) *[]stable.ObjectMapping {
	if mappings.IsNull() || mappings.IsUnknown() {
		return nil
	}

	var mappingModels []ObjectMappingModel
	diags.Append(mappings.ElementsAs(ctx, &mappingModels, false)...)
	if diags.HasError() {
		return nil
	}

	expandedMappings := make([]stable.ObjectMapping, 0)

	for _, mapping := range mappingModels {
		expandedMapping := stable.ObjectMapping{
			Enabled:          pointer.To(mapping.Enabled.ValueBool()),
			Name:             nullable.Value(mapping.Name.ValueString()),
			SourceObjectName: nullable.Value(mapping.SourceObjectName.ValueString()),
			TargetObjectName: nullable.Value(mapping.TargetObjectName.ValueString()),
		}

		if !mapping.FlowTypes.IsNull() && !mapping.FlowTypes.IsUnknown() {
			flowTypes := stable.ObjectFlowTypes(mapping.FlowTypes.ValueString())
			expandedMapping.FlowTypes = &flowTypes
		}

		if !mapping.Attributes.IsNull() && !mapping.Attributes.IsUnknown() {
			expandedMapping.AttributeMappings = expandAttributes(ctx, mapping.Attributes, diags)
		}

		expandedMappings = append(expandedMappings, expandedMapping)
	}

	return &expandedMappings
}

func expandAttributes(ctx context.Context, attributes types.List, diags *diag.Diagnostics) *[]stable.AttributeMapping {
	if attributes.IsNull() || attributes.IsUnknown() {
		return nil
	}

	var attributeModels []AttributeModel
	diags.Append(attributes.ElementsAs(ctx, &attributeModels, false)...)
	if diags.HasError() {
		return nil
	}

	expandedAttributes := make([]stable.AttributeMapping, 0)

	for _, attr := range attributeModels {
		expandedAttr := stable.AttributeMapping{
			TargetAttributeName: nullable.Value(attr.TargetAttributeName.ValueString()),
		}

		if !attr.DefaultValue.IsNull() && !attr.DefaultValue.IsUnknown() {
			expandedAttr.DefaultValue = nullable.Value(attr.DefaultValue.ValueString())
		}

		if !attr.ExportMissingReferences.IsNull() && !attr.ExportMissingReferences.IsUnknown() {
			expandedAttr.ExportMissingReferences = pointer.To(attr.ExportMissingReferences.ValueBool())
		}

		if !attr.FlowBehavior.IsNull() && !attr.FlowBehavior.IsUnknown() {
			flowBehavior := stable.AttributeFlowBehavior(attr.FlowBehavior.ValueString())
			expandedAttr.FlowBehavior = &flowBehavior
		}

		if !attr.FlowType.IsNull() && !attr.FlowType.IsUnknown() {
			flowType := stable.AttributeFlowType(attr.FlowType.ValueString())
			expandedAttr.FlowType = &flowType
		}

		if !attr.MatchingPriority.IsNull() && !attr.MatchingPriority.IsUnknown() {
			expandedAttr.MatchingPriority = pointer.To(attr.MatchingPriority.ValueInt64())
		}

		if attr.Source != nil {
			expandedAttr.Source = expandSource(ctx, *attr.Source, diags)
		}

		expandedAttributes = append(expandedAttributes, expandedAttr)
	}

	return &expandedAttributes
}

func expandSource(ctx context.Context, source AttributeSourceModel, diags *diag.Diagnostics) *stable.AttributeMappingSource {
	expandedSource := stable.AttributeMappingSource{}

	if !source.Expression.IsNull() && !source.Expression.IsUnknown() {
		expandedSource.Expression = nullable.Value(source.Expression.ValueString())
	}

	if !source.Name.IsNull() && !source.Name.IsUnknown() {
		expandedSource.Name = nullable.Value(source.Name.ValueString())
	}

	if !source.Type.IsNull() && !source.Type.IsUnknown() {
		sourceType := stable.AttributeMappingSourceType(source.Type.ValueString())
		expandedSource.Type = &sourceType
	}

	if !source.Parameters.IsNull() && !source.Parameters.IsUnknown() {
		expandedSource.Parameters = expandParameters(ctx, source.Parameters, diags)
	}

	return &expandedSource
}

func expandParameters(ctx context.Context, parameters types.List, diags *diag.Diagnostics) *[]stable.StringKeyAttributeMappingSourceValuePair {
	if parameters.IsNull() || parameters.IsUnknown() {
		return nil
	}

	var paramModels []AttributeSourceParameterModel
	diags.Append(parameters.ElementsAs(ctx, &paramModels, false)...)
	if diags.HasError() {
		return nil
	}

	paramPairs := make([]stable.StringKeyAttributeMappingSourceValuePair, 0)
	for _, param := range paramModels {
		paramPair := stable.StringKeyAttributeMappingSourceValuePair{
			Key: nullable.Value(param.Key.ValueString()),
		}

		if param.Value != nil {
			src := &stable.AttributeMappingSource{}
			if !param.Value.Type.IsNull() && !param.Value.Type.IsUnknown() {
				sourceType := stable.AttributeMappingSourceType(param.Value.Type.ValueString())
				src.Type = &sourceType
			}
			if !param.Value.Name.IsNull() && !param.Value.Name.IsUnknown() {
				src.Name = nullable.Value(param.Value.Name.ValueString())
			}
			if !param.Value.Expression.IsNull() && !param.Value.Expression.IsUnknown() {
				src.Expression = nullable.Value(param.Value.Expression.ValueString())
			}

			paramPair.Value = src
		}

		paramPairs = append(paramPairs, paramPair)
	}

	return &paramPairs
}

func flattenSynchronizationRules(rules *[]stable.SynchronizationRule) types.List {
	if rules == nil || len(*rules) == 0 {
		return types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"id":                    types.StringType,
				"name":                  types.StringType,
				"source_directory_name": types.StringType,
				"target_directory_name": types.StringType,
				"object_mapping": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"enabled":            types.BoolType,
						"flow_types":         types.StringType,
						"name":               types.StringType,
						"source_object_name": types.StringType,
						"target_object_name": types.StringType,
						"attribute": types.ListType{ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"default_value":             types.StringType,
								"export_missing_references": types.BoolType,
								"flow_behavior":             types.StringType,
								"flow_type":                 types.StringType,
								"matching_priority":         types.Int64Type,
								"target_attribute_name":     types.StringType,
								"source": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"expression": types.StringType,
										"name":       types.StringType,
										"parameters": types.ListType{ElemType: types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"key": types.StringType,
												"value": types.ObjectType{
													AttrTypes: map[string]attr.Type{
														"expression": types.StringType,
														"name":       types.StringType,
														"type":       types.StringType,
													},
												},
											},
										}},
										"type": types.StringType,
									},
								},
							},
						}},
					},
				}},
			},
		})
	}

	flattenedRules := make([]attr.Value, 0)

	for _, rule := range *rules {
		flattenedRule := map[string]attr.Value{
			"name":                  types.StringValue(rule.Name.GetOrZero()),
			"source_directory_name": types.StringValue(rule.SourceDirectoryName.GetOrZero()),
			"target_directory_name": types.StringValue(rule.TargetDirectoryName.GetOrZero()),
		}

		if rule.Id.IsSet() {
			flattenedRule["id"] = types.StringValue(rule.Id.GetOrZero())
		} else {
			flattenedRule["id"] = types.StringNull()
		}

		if rule.ObjectMappings != nil {
			flattenedRule["object_mapping"] = flattenObjectMappings(rule.ObjectMappings)
		} else {
			flattenedRule["object_mapping"] = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"enabled":            types.BoolType,
					"flow_types":         types.StringType,
					"name":               types.StringType,
					"source_object_name": types.StringType,
					"target_object_name": types.StringType,
					"attribute": types.ListType{ElemType: types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"default_value":             types.StringType,
							"export_missing_references": types.BoolType,
							"flow_behavior":             types.StringType,
							"flow_type":                 types.StringType,
							"matching_priority":         types.Int64Type,
							"target_attribute_name":     types.StringType,
							"source": types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"expression": types.StringType,
									"name":       types.StringType,
									"parameters": types.ListType{ElemType: types.ObjectType{
										AttrTypes: map[string]attr.Type{
											"key": types.StringType,
											"value": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"expression": types.StringType,
													"name":       types.StringType,
													"type":       types.StringType,
												},
											},
										},
									}},
									"type": types.StringType,
								},
							},
						},
					}},
				},
			})
		}

		flattenedRules = append(flattenedRules, types.ObjectValueMust(
			map[string]attr.Type{
				"id":                    types.StringType,
				"name":                  types.StringType,
				"source_directory_name": types.StringType,
				"target_directory_name": types.StringType,
				"object_mapping": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"enabled":            types.BoolType,
						"flow_types":         types.StringType,
						"name":               types.StringType,
						"source_object_name": types.StringType,
						"target_object_name": types.StringType,
						"attribute": types.ListType{ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"default_value":             types.StringType,
								"export_missing_references": types.BoolType,
								"flow_behavior":             types.StringType,
								"flow_type":                 types.StringType,
								"matching_priority":         types.Int64Type,
								"target_attribute_name":     types.StringType,
								"source": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"expression": types.StringType,
										"name":       types.StringType,
										"parameters": types.ListType{ElemType: types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"key": types.StringType,
												"value": types.ObjectType{
													AttrTypes: map[string]attr.Type{
														"expression": types.StringType,
														"name":       types.StringType,
														"type":       types.StringType,
													},
												},
											},
										}},
										"type": types.StringType,
									},
								},
							},
						}},
					},
				}},
			},
			flattenedRule,
		))
	}

	return types.ListValueMust(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"id":                    types.StringType,
				"name":                  types.StringType,
				"source_directory_name": types.StringType,
				"target_directory_name": types.StringType,
				"object_mapping": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"enabled":            types.BoolType,
						"flow_types":         types.StringType,
						"name":               types.StringType,
						"source_object_name": types.StringType,
						"target_object_name": types.StringType,
						"attribute": types.ListType{ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"default_value":             types.StringType,
								"export_missing_references": types.BoolType,
								"flow_behavior":             types.StringType,
								"flow_type":                 types.StringType,
								"matching_priority":         types.Int64Type,
								"target_attribute_name":     types.StringType,
								"source": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"expression": types.StringType,
										"name":       types.StringType,
										"parameters": types.ListType{ElemType: types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"key": types.StringType,
												"value": types.ObjectType{
													AttrTypes: map[string]attr.Type{
														"expression": types.StringType,
														"name":       types.StringType,
														"type":       types.StringType,
													},
												},
											},
										}},
										"type": types.StringType,
									},
								},
							},
						}},
					},
				}},
			},
		},
		flattenedRules,
	)
}

func flattenObjectMappings(mappings *[]stable.ObjectMapping) types.List {
	if mappings == nil || len(*mappings) == 0 {
		return types.ListNull(types.ObjectType{})
	}

	flattenedMappings := make([]attr.Value, 0)

	for _, mapping := range *mappings {
		flattenedMapping := map[string]attr.Value{
			"enabled":            types.BoolValue(pointer.From(mapping.Enabled)),
			"name":               types.StringValue(mapping.Name.GetOrZero()),
			"source_object_name": types.StringValue(mapping.SourceObjectName.GetOrZero()),
			"target_object_name": types.StringValue(mapping.TargetObjectName.GetOrZero()),
		}

		if mapping.FlowTypes != nil {
			flattenedMapping["flow_types"] = types.StringValue(string(*mapping.FlowTypes))
		} else {
			flattenedMapping["flow_types"] = types.StringNull()
		}

		if mapping.AttributeMappings != nil {
			flattenedMapping["attribute"] = flattenAttributes(mapping.AttributeMappings)
		} else {
			flattenedMapping["attribute"] = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"default_value":             types.StringType,
					"export_missing_references": types.BoolType,
					"flow_behavior":             types.StringType,
					"flow_type":                 types.StringType,
					"matching_priority":         types.Int64Type,
					"target_attribute_name":     types.StringType,
					"source": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"expression": types.StringType,
							"name":       types.StringType,
							"parameters": types.ListType{ElemType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"key": types.StringType,
									"value": types.ObjectType{
										AttrTypes: map[string]attr.Type{
											"expression": types.StringType,
											"name":       types.StringType,
											"type":       types.StringType,
										},
									},
								},
							}},
							"type": types.StringType,
						},
					},
				},
			})
		}

		flattenedMappings = append(flattenedMappings, types.ObjectValueMust(
			map[string]attr.Type{
				"enabled":            types.BoolType,
				"flow_types":         types.StringType,
				"name":               types.StringType,
				"source_object_name": types.StringType,
				"target_object_name": types.StringType,
				"attribute": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"default_value":             types.StringType,
						"export_missing_references": types.BoolType,
						"flow_behavior":             types.StringType,
						"flow_type":                 types.StringType,
						"matching_priority":         types.Int64Type,
						"target_attribute_name":     types.StringType,
						"source": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"expression": types.StringType,
								"name":       types.StringType,
								"parameters": types.ListType{ElemType: types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"key": types.StringType,
										"value": types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"expression": types.StringType,
												"name":       types.StringType,
												"type":       types.StringType,
											},
										},
									},
								}},
								"type": types.StringType,
							},
						},
					},
				}},
			},
			flattenedMapping,
		))
	}

	return types.ListValueMust(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"enabled":            types.BoolType,
				"flow_types":         types.StringType,
				"name":               types.StringType,
				"source_object_name": types.StringType,
				"target_object_name": types.StringType,
				"attribute": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"default_value":             types.StringType,
						"export_missing_references": types.BoolType,
						"flow_behavior":             types.StringType,
						"flow_type":                 types.StringType,
						"matching_priority":         types.Int64Type,
						"target_attribute_name":     types.StringType,
						"source": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"expression": types.StringType,
								"name":       types.StringType,
								"parameters": types.ListType{ElemType: types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"key": types.StringType,
										"value": types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"expression": types.StringType,
												"name":       types.StringType,
												"type":       types.StringType,
											},
										},
									},
								}},
								"type": types.StringType,
							},
						},
					},
				}},
			},
		},
		flattenedMappings,
	)
}

func flattenAttributes(attributes *[]stable.AttributeMapping) types.List {
	if attributes == nil || len(*attributes) == 0 {
		return types.ListNull(types.ObjectType{})
	}

	flattenedAttributes := make([]attr.Value, 0)

	for _, attribute := range *attributes {
		flattenedAttr := map[string]attr.Value{
			"target_attribute_name": types.StringValue(attribute.TargetAttributeName.GetOrZero()),
		}

		if attribute.DefaultValue.IsSet() {
			if v := attribute.DefaultValue.Get(); v != nil {
				flattenedAttr["default_value"] = types.StringValue(*v)
			} else {
				flattenedAttr["default_value"] = types.StringNull()
			}

		} else {
			flattenedAttr["default_value"] = types.StringNull()
		}

		if attribute.ExportMissingReferences != nil {
			flattenedAttr["export_missing_references"] = types.BoolValue(*attribute.ExportMissingReferences)
		} else {
			flattenedAttr["export_missing_references"] = types.BoolNull()
		}

		if attribute.FlowBehavior != nil {
			flattenedAttr["flow_behavior"] = types.StringValue(string(*attribute.FlowBehavior))
		} else {
			flattenedAttr["flow_behavior"] = types.StringNull()
		}

		if attribute.FlowType != nil {
			flattenedAttr["flow_type"] = types.StringValue(string(*attribute.FlowType))
		} else {
			flattenedAttr["flow_type"] = types.StringNull()
		}

		if attribute.MatchingPriority != nil {
			flattenedAttr["matching_priority"] = types.Int64Value(*attribute.MatchingPriority)
		} else {
			flattenedAttr["matching_priority"] = types.Int64Null()
		}

		if attribute.Source != nil {
			flattenedAttr["source"] = flattenSource(attribute.Source)
		} else {
			flattenedAttr["source"] = types.ObjectNull(map[string]attr.Type{
				"expression": types.StringType,
				"name":       types.StringType,
				"parameters": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"key": types.StringType,
						"value": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"expression": types.StringType,
								"name":       types.StringType,
								"type":       types.StringType,
							},
						},
					},
				}},
				"type": types.StringType,
			})
		}

		flattenedAttributes = append(flattenedAttributes, types.ObjectValueMust(
			map[string]attr.Type{
				"default_value":             types.StringType,
				"export_missing_references": types.BoolType,
				"flow_behavior":             types.StringType,
				"flow_type":                 types.StringType,
				"matching_priority":         types.Int64Type,
				"target_attribute_name":     types.StringType,
				"source": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"expression": types.StringType,
						"name":       types.StringType,
						"parameters": types.ListType{ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"key": types.StringType,
								"value": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"expression": types.StringType,
										"name":       types.StringType,
										"type":       types.StringType,
									},
								},
							},
						}},
						"type": types.StringType,
					},
				},
			},
			flattenedAttr,
		))
	}

	return types.ListValueMust(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"default_value":             types.StringType,
				"export_missing_references": types.BoolType,
				"flow_behavior":             types.StringType,
				"flow_type":                 types.StringType,
				"matching_priority":         types.Int64Type,
				"target_attribute_name":     types.StringType,
				"source": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"expression": types.StringType,
						"name":       types.StringType,
						"parameters": types.ListType{ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"key": types.StringType,
								"value": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"expression": types.StringType,
										"name":       types.StringType,
										"type":       types.StringType,
									},
								},
							},
						}},
						"type": types.StringType,
					},
				},
			},
		},
		flattenedAttributes,
	)
}

func flattenParameters(parameters *[]stable.StringKeyAttributeMappingSourceValuePair) types.List {
	if parameters == nil || len(*parameters) == 0 {
		return types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"key": types.StringType,
				"value": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"expression": types.StringType,
						"name":       types.StringType,
						"type":       types.StringType,
					},
				},
			},
		})
	}

	flattenedParams := make([]attr.Value, 0)

	for _, param := range *parameters {
		flattenedParam := map[string]attr.Value{
			"key": types.StringValue(param.Key.GetOrZero()),
		}

		if param.Value != nil {
			value := map[string]attr.Value{}
			if param.Value.Type != nil {
				value["type"] = types.StringValue(string(*param.Value.Type))
			} else {
				value["type"] = types.StringNull()
			}
			if param.Value.Name.IsSet() {
				value["name"] = types.StringValue(param.Value.Name.GetOrZero())
			} else {
				value["name"] = types.StringNull()
			}
			if param.Value.Expression.IsSet() {
				value["expression"] = types.StringValue(param.Value.Expression.GetOrZero())
			} else {
				value["expression"] = types.StringNull()
			}
			flattenedParam["value"] = types.ObjectValueMust(
				map[string]attr.Type{
					"expression": types.StringType,
					"name":       types.StringType,
					"type":       types.StringType,
				}, value)

		} else {
			flattenedParam["value"] = types.ObjectNull(map[string]attr.Type{
				"expression": types.StringType,
				"name":       types.StringType,
				"type":       types.StringType,
			})
		}

		flattenedParams = append(flattenedParams, types.ObjectValueMust(
			map[string]attr.Type{
				"key": types.StringType,
				"value": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"expression": types.StringType,
						"name":       types.StringType,
						"type":       types.StringType,
					},
				},
			},
			flattenedParam,
		))
	}

	return types.ListValueMust(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"key": types.StringType,
				"value": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"expression": types.StringType,
						"name":       types.StringType,
						"type":       types.StringType,
					},
				},
			},
		},
		flattenedParams,
	)
}

func flattenSource(source *stable.AttributeMappingSource) types.Object {
	if source == nil {
		return types.ObjectNull(map[string]attr.Type{
			"expression": types.StringType,
			"name":       types.StringType,
			"parameters": types.ListType{ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"key": types.StringType,
					"value": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"expression": types.StringType,
							"name":       types.StringType,
							"type":       types.StringType,
						},
					},
				},
			}},
			"type": types.StringType,
		})
	}

	flattenedSource := map[string]attr.Value{}

	if source.Expression.IsSet() {
		flattenedSource["expression"] = types.StringValue(source.Expression.GetOrZero())
	} else {
		flattenedSource["expression"] = types.StringNull()
	}

	if source.Name.IsSet() {
		flattenedSource["name"] = types.StringValue(source.Name.GetOrZero())
	} else {
		flattenedSource["name"] = types.StringNull()
	}

	if source.Type != nil {
		flattenedSource["type"] = types.StringValue(string(*source.Type))
	} else {
		flattenedSource["type"] = types.StringNull()
	}

	if source.Parameters != nil {
		flattenedSource["parameters"] = flattenParameters(source.Parameters)
	} else {
		flattenedSource["parameters"] = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"key": types.StringType,
				"value": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"expression": types.StringType,
						"name":       types.StringType,
						"type":       types.StringType,
					},
				},
			},
		})
	}

	return types.ObjectValueMust(
		map[string]attr.Type{
			"expression": types.StringType,
			"name":       types.StringType,
			"parameters": types.ListType{ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"key": types.StringType,
					"value": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"expression": types.StringType,
							"name":       types.StringType,
							"type":       types.StringType,
						},
					},
				},
			}},
			"type": types.StringType,
		},
		flattenedSource,
	)
}
