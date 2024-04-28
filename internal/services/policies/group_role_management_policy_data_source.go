// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policies

import (
	"context"
	"errors"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/sdk"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf"
	"time"

	"github.com/hashicorp/terraform-provider-azuread/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf/validation"
	"github.com/manicminer/hamilton/msgraph"
)

func groupRoleManagementPolicyDataSource() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		ReadContext: groupRoleManagementPolicyDataSourceRead,

		Timeouts: &pluginsdk.ResourceTimeout{
			Read: pluginsdk.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"group_id": {
				Description:      "ID of the group to which this policy is assigned",
				Type:             pluginsdk.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validation.ValidateDiag(validation.IsUUID),
			},

			"role_id": {
				Description: "The ID of the role of this policy to the group",
				Type:        pluginsdk.TypeString,
				Required:    true,
				ForceNew:    true,
				ValidateDiagFunc: validation.ValidateDiag(validation.StringInSlice([]string{
					msgraph.PrivilegedAccessGroupRelationshipMember,
					msgraph.PrivilegedAccessGroupRelationshipOwner,
					msgraph.PrivilegedAccessGroupRelationshipUnknown,
				}, false)),
			},
		},
	}
}

func groupRoleManagementPolicyDataSourceRead(ctx context.Context, d *pluginsdk.ResourceData, meta interface{}) pluginsdk.Diagnostics {
	client := meta.(*clients.Client).Groups.GroupsClient
	client.BaseClient.DisableRetries = true
	defer func() { client.BaseClient.DisableRetries = false }()

	groupID := d.Get("group_id").(string)
	roleID := d.Get("role_id").(string)
	id, err := getPolicyId(ctx, sdk.ResourceMetaData{Client: meta.(*clients.Client)}, groupID, roleID)
	if err != nil {
		return tf.ErrorDiagF(errors.New("Bad API response"), "ID is nil for returned Group Role Management Policy", "group_id", groupID, "role_id", roleID)
	}
	d.SetId(id.ID())
	return nil
}
