package framework

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/go-azure-sdk/sdk/auth"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/services/synchronization"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Terraform's Microsoft Partner ID is this specific GUID
const terraformPartnerId = "222c6c49-1b0a-5959-a213-6608f9eb8820"

// Ensure the implementation satisfies the expected interfaces
var (
	_ provider.Provider = &adProvider{}
)

// client satisfies the provider.Provider interface and usually is included
// with all Resource and DataSource implementations.
type adProvider struct {
	client  *clients.Client
	version string
}

// providerData can be used to store data from the Terraform configuration.
type providerData struct {
	ClientID                       types.String `tfsdk:"client_id"`
	ClientIDFilePath               types.String `tfsdk:"client_id_file_path"`
	TenantID                       types.String `tfsdk:"tenant_id"`
	Environment                    types.String `tfsdk:"environment"`
	MetadataHost                   types.String `tfsdk:"metadata_host"`
	ClientCertificate              types.String `tfsdk:"client_certificate"`
	ClientCertificatePassword      types.String `tfsdk:"client_certificate_password"`
	ClientCertificatePath          types.String `tfsdk:"client_certificate_path"`
	ClientSecret                   types.String `tfsdk:"client_secret"`
	ClientSecretFilePath           types.String `tfsdk:"client_secret_file_path"`
	UseOIDC                        types.Bool   `tfsdk:"use_oidc"`
	OIDCToken                      types.String `tfsdk:"oidc_token"`
	OIDCTokenFilePath              types.String `tfsdk:"oidc_token_file_path"`
	ADOPipelineServiceConnectionID types.String `tfsdk:"ado_pipeline_service_connection_id"`
	OIDCRequestToken               types.String `tfsdk:"oidc_request_token"`
	OIDCRequestURL                 types.String `tfsdk:"oidc_request_url"`
	UseAKSWorkloadIdentity         types.Bool   `tfsdk:"use_aks_workload_identity"`
	UseCLI                         types.Bool   `tfsdk:"use_cli"`
	UseMSI                         types.Bool   `tfsdk:"use_msi"`
	MSIEndpoint                    types.String `tfsdk:"msi_endpoint"`
	PartnerID                      types.String `tfsdk:"partner_id"`
	DisableTerraformPartnerID      types.Bool   `tfsdk:"disable_terraform_partner_id"`
}

// Metadata returns the client type name.
func (p *adProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "azuread"
}

func (p *adProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var certData []byte
	var data providerData
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if v := os.Getenv("ARM_CLIENT_ID"); v != "" && data.ClientID.IsNull() {
		data.ClientID = types.StringValue(v)
	}

	if v := os.Getenv("ARM_CLIENT_ID_FILE_PATH"); v != "" && data.ClientIDFilePath.IsNull() {
		data.ClientIDFilePath = types.StringValue(v)
	}

	if v := os.Getenv("ARM_TENANT_ID"); v != "" && data.TenantID.IsNull() {
		data.TenantID = types.StringValue(v)
	}

	if v := os.Getenv("ARM_ENVIRONMENT"); data.Environment.IsNull() {
		data.Environment = types.StringValue(cmp.Or(v, "global"))
	}

	if v := os.Getenv("ARM_METADATA_HOSTNAME"); v != "" && data.MetadataHost.IsNull() {
		data.MetadataHost = types.StringValue(v)
	}

	if v := os.Getenv("ARM_CLIENT_CERTIFICATE"); v != "" && data.ClientCertificate.IsNull() {
		data.ClientCertificate = types.StringValue(v)
	}

	if v := os.Getenv("ARM_CLIENT_CERTIFICATE_PASSWORD"); v != "" && data.ClientCertificatePassword.IsNull() {
		data.ClientCertificatePassword = types.StringValue(v)
	}

	if v := os.Getenv("ARM_CLIENT_CERTIFICATE_PATH"); v != "" && data.ClientCertificatePath.IsNull() {
		data.ClientCertificatePath = types.StringValue(v)
	}

	if v := os.Getenv("ARM_CLIENT_SECRET"); v != "" && data.ClientSecret.IsNull() {
		data.ClientSecret = types.StringValue(v)
	}

	if v := os.Getenv("ARM_CLIENT_SECRET_FILE_PATH"); v != "" && data.ClientSecretFilePath.IsNull() {
		data.ClientSecretFilePath = types.StringValue(v)
	}

	if v := os.Getenv("ARM_USE_OIDC"); v != "" && data.UseOIDC.IsNull() {
		parsed, _ := strconv.ParseBool(v)
		data.UseOIDC = types.BoolValue(parsed)
	}

	if v := os.Getenv("ARM_OIDC_TOKEN"); v != "" && data.OIDCToken.IsNull() {
		data.OIDCToken = types.StringValue(v)
	}

	if v := os.Getenv("ARM_OIDC_TOKEN_FILE_PATH"); v != "" && data.OIDCTokenFilePath.IsNull() {
		data.OIDCTokenFilePath = types.StringValue(v)
	}

	if data.ADOPipelineServiceConnectionID.IsNull() {
		data.ADOPipelineServiceConnectionID = types.StringValue(cmp.Or(os.Getenv("ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID"), os.Getenv("ARM_OIDC_AZURE_SERVICE_CONNECTION_ID"), ""))
	}

	if data.OIDCRequestToken.IsNull() {
		data.OIDCRequestToken = types.StringValue(cmp.Or(os.Getenv("ARM_OIDC_REQUEST_TOKEN"), os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"), os.Getenv("SYSTEM_ACCESSTOKEN"), ""))
	}

	if data.OIDCRequestURL.IsNull() {
		data.OIDCRequestURL = types.StringValue(cmp.Or(os.Getenv("ARM_OIDC_REQUEST_URL"), os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), os.Getenv("SYSTEM_OIDCREQUESTURI"), ""))
	}

	if v := os.Getenv("ARM_USE_AKS_WORKLOAD_IDENTITY"); v != "" && data.UseAKSWorkloadIdentity.IsNull() {
		parsed, _ := strconv.ParseBool(v)
		data.UseAKSWorkloadIdentity = types.BoolValue(parsed)
	}

	if v := os.Getenv("ARM_USE_CLI"); data.UseCLI.IsNull() {
		if v != "" {
			if parsed, err := strconv.ParseBool(v); err == nil {
				data.UseCLI = types.BoolValue(parsed)
			}
		} else {
			data.UseCLI = types.BoolValue(true) // default fallback
		}
	}

	if v := os.Getenv("ARM_USE_MSI"); v != "" && data.UseMSI.IsNull() {
		parsed, _ := strconv.ParseBool(v)
		data.UseMSI = types.BoolValue(parsed)
	}

	if v := os.Getenv("ARM_MSI_ENDPOINT"); v != "" && data.MSIEndpoint.IsNull() {
		data.MSIEndpoint = types.StringValue(v)
	}

	if v := os.Getenv("ARM_PARTNER_ID"); v != "" && data.PartnerID.IsNull() {
		data.PartnerID = types.StringValue(v)
	}

	if v := os.Getenv("ARM_DISABLE_TERRAFORM_PARTNER_ID"); v != "" && data.DisableTerraformPartnerID.IsNull() {
		parsed, _ := strconv.ParseBool(v)
		data.DisableTerraformPartnerID = types.BoolValue(parsed)
	}

	if !data.ClientCertificate.IsNull() {
		var err error
		tflog.Info(ctx, "decoding certificate")
		certData, err = decodeCertificate(data.ClientCertificate.String())
		if err != nil {
			resp.Diagnostics.AddError(err.Error(), err.Error())
			return
		}
	}

	idToken, err := getOidcToken(data)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), err.Error())
		return
	}

	clientSecret, err := getClientSecret(data)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), err.Error())
		return
	}

	clientId, err := getClientId(data)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), err.Error())
		return
	}

	tenantId, err := getTenantId(data)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), err.Error())
		return
	}

	var (
		env *environments.Environment

		envName      = data.Environment.ValueString()
		metadataHost = data.MetadataHost.ValueString()
	)

	if metadataHost != "" {
		logEntry("[DEBUG] Configuring cloud environment from Metadata Service at %q", metadataHost)
		if env, err = environments.FromEndpoint(ctx, fmt.Sprintf("https://%s", metadataHost)); err != nil {
			resp.Diagnostics.AddError(err.Error(), err.Error())
			return
		}
	} else {
		logEntry("[DEBUG] Configuring built-in cloud environment by name: %q", envName)
		if env, err = environments.FromName(envName); err != nil {
			resp.Diagnostics.AddError(err.Error(), err.Error())
			return
		}
	}

	if env.MicrosoftGraph == nil {
		resp.Diagnostics.AddError("Microsoft Graph was not configured for the specified environment", "")
		return
	} else if endpoint, ok := env.MicrosoftGraph.Endpoint(); !ok || *endpoint == "" {
		resp.Diagnostics.AddError("Microsoft Graph endpoint could not be determined for the specified environment", "")
		return
	}

	var (
		enableAzureCli        = data.UseCLI.ValueBool()
		enableManagedIdentity = data.UseMSI.ValueBool()
		enableOidc            = data.UseOIDC.ValueBool() || data.UseAKSWorkloadIdentity.ValueBool()
	)

	authConfig := &auth.Credentials{
		Environment: *env,
		ClientID:    *clientId,
		TenantID:    *tenantId,

		ClientCertificateData:     certData,
		ClientCertificatePassword: data.ClientCertificatePassword.ValueString(),
		ClientCertificatePath:     data.ClientCertificatePath.ValueString(),
		ClientSecret:              *clientSecret,

		OIDCAssertionToken:             *idToken,
		OIDCTokenRequestURL:            data.OIDCRequestURL.ValueString(),
		OIDCTokenRequestToken:          data.OIDCRequestToken.ValueString(),
		ADOPipelineServiceConnectionID: data.ADOPipelineServiceConnectionID.ValueString(),

		CustomManagedIdentityEndpoint: data.MSIEndpoint.ValueString(),

		EnableAuthenticatingUsingAzureCLI:          enableAzureCli,
		EnableAuthenticatingUsingClientCertificate: true,
		EnableAuthenticatingUsingClientSecret:      true,
		EnableAuthenticatingUsingManagedIdentity:   enableManagedIdentity,
		EnableAuthenticationUsingGitHubOIDC:        enableOidc,
		EnableAuthenticationUsingADOPipelineOIDC:   enableOidc,
		EnableAuthenticationUsingOIDC:              enableOidc,
	}

	// only one pid can be interpreted currently
	// hence, send partner ID if present, otherwise send Terraform GUID
	// unless users have opted out
	partnerId := data.PartnerID.ValueString()
	if partnerId == "" && !data.DisableTerraformPartnerID.ValueBool() {
		partnerId = terraformPartnerId
	}

	p.client, err = buildClient(ctx, req, authConfig, partnerId)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), "")
	}

	resp.ResourceData = p.client
	resp.DataSourceData = p.client
}

func buildClient(ctx context.Context, req provider.ConfigureRequest, authConfig *auth.Credentials, partnerId string) (*clients.Client, error) {
	clientBuilder := clients.ClientBuilder{
		AuthConfig:       authConfig,
		PartnerID:        partnerId,
		TerraformVersion: req.TerraformVersion,
	}

	client, err := clientBuilder.Build(ctx)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (p *adProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		synchronization.NewSynchronizationJobSchemaResource,
	}
}

func (p *adProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *adProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_ID", ""),
				Description: "The Client ID which should be used for service principal authentication",
			},

			"client_id_file_path": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_ID_FILE_PATH", ""),
				Description: "The path to a file containing the Client ID which should be used for service principal authentication",
			},

			"tenant_id": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_TENANT_ID", ""),
				Description: "The Tenant ID which should be used. Works with all authentication methods except Managed Identity",
			},

			"environment": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_ENVIRONMENT", "global"),
				Description: "The cloud environment which should be used. Possible values are: `global` (also `public`), `usgovernmentl4` (also `usgovernment`), `usgovernmentl5` (also `dod`), and `china`. Defaults to `global`. Not used and should not be specified when `metadata_host` is specified.",
			},

			"metadata_host": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_METADATA_HOSTNAME", ""),
				Description: "The Hostname which should be used for the Azure Metadata Service.",
			},

			// Client Certificate specific fields
			"client_certificate": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_CERTIFICATE", ""),
				Description: "Base64 encoded PKCS#12 certificate bundle to use when authenticating as a Service Principal using a Client Certificate",
			},

			"client_certificate_password": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_CERTIFICATE_PASSWORD", ""),
				Description: "The password to decrypt the Client Certificate. For use when authenticating as a Service Principal using a Client Certificate",
			},

			"client_certificate_path": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_CERTIFICATE_PATH", ""),
				Description: "The path to the Client Certificate associated with the Service Principal for use when authenticating as a Service Principal using a Client Certificate",
			},

			// Client Secret specific fields
			"client_secret": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_SECRET", ""),
				Description: "The application password to use when authenticating as a Service Principal using a Client Secret",
			},

			"client_secret_file_path": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_CLIENT_SECRET_FILE_PATH", ""),
				Description: "The path to a file containing the application password to use when authenticating as a Service Principal using a Client Secret",
			},

			// OIDC specific fields
			"use_oidc": schema.BoolAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_USE_OIDC", false),
				Description: "Allow OpenID Connect to be used for authentication",
			},

			"oidc_token": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_OIDC_TOKEN", ""),
				Description: "The ID token for use when authenticating as a Service Principal using OpenID Connect.",
			},

			"oidc_token_file_path": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_OIDC_TOKEN_FILE_PATH", ""),
				Description: "The path to a file containing an ID token for use when authenticating as a Service Principal using OpenID Connect.",
			},

			"ado_pipeline_service_connection_id": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: schema.MultiEnvDefaultFunc([]string{"ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID", "ARM_OIDC_AZURE_SERVICE_CONNECTION_ID"}, nil),
				Description: "The Azure DevOps Pipeline Service Connection ID.",
			},

			"oidc_request_token": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.MultiEnvDefaultFunc([]string{"ARM_OIDC_REQUEST_TOKEN", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "SYSTEM_ACCESSTOKEN"}, ""),
				Description: "The bearer token for the request to the OIDC provider. For use when authenticating as a Service Principal using OpenID Connect.",
			},

			"oidc_request_url": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.MultiEnvDefaultFunc([]string{"ARM_OIDC_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_URL", "SYSTEM_OIDCREQUESTURI"}, ""),
				Description: "The URL for the OIDC provider from which to request an ID token. For use when authenticating as a Service Principal using OpenID Connect.",
			},

			// Azure AKS Workload Identity fields
			"use_aks_workload_identity": schema.BoolAttribute{
				Optional: true,
				//DefaultFunc: schema.EnvDefaultFunc("ARM_USE_AKS_WORKLOAD_IDENTITY", false),
				Description: "Allow Azure AKS Workload Identity to be used for Authentication.",
			},

			// CLI authentication specific fields
			"use_cli": schema.BoolAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_USE_CLI", true),
				Description: "Allow Azure CLI to be used for Authentication",
			},

			// Managed Identity specific fields
			"use_msi": schema.BoolAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_USE_MSI", false),
				Description: "Allow Managed Identity to be used for Authentication",
			},

			"msi_endpoint": schema.StringAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_MSI_ENDPOINT", ""),
				Description: "The path to a custom endpoint for Managed Identity - in most circumstances this should be detected automatically",
			},

			// Managed Tracking GUID for User-agent
			"partner_id": schema.StringAttribute{
				Optional: true,
				//ValidateFunc: validation.Any(validation.IsUUID, validation.StringIsEmpty),
				//DefaultFunc:  pluginsdk.EnvDefaultFunc("ARM_PARTNER_ID", ""),
				Validators:  []validator.String{},
				Description: "A GUID/UUID that is registered with Microsoft to facilitate partner resource usage attribution",
			},

			"disable_terraform_partner_id": schema.BoolAttribute{
				Optional: true,
				//DefaultFunc: pluginsdk.EnvDefaultFunc("ARM_DISABLE_TERRAFORM_PARTNER_ID", false),
				Description: "Disable the Terraform Partner ID, which is used if a custom `partner_id` isn't specified",
			},
		},
	}
}

func New(version string) provider.Provider {
	return &adProvider{
		version: version,
	}
}
