// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-azure-sdk/sdk/auth"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/helpers/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azuread/internal/helpers/tf/validation"
	"github.com/hashicorp/terraform-provider-azuread/internal/sdk"
)

// Terraform's Microsoft Partner ID is this specific GUID
const terraformPartnerId = "222c6c49-1b0a-5959-a213-6608f9eb8820"

type ServiceRegistration interface {
	// Name is the name of this Service
	Name() string

	// WebsiteCategories returns a list of categories which can be used for the sidebar
	WebsiteCategories() []string

	// SupportedDataSources returns the supported Data Sources supported by this Service
	SupportedDataSources() map[string]*pluginsdk.Resource

	// SupportedResources returns the supported Resources supported by this Service
	SupportedResources() map[string]*pluginsdk.Resource
}

// AzureADProvider returns a schema.Provider.
func AzureADProvider() *schema.Provider {
	dataSources := make(map[string]*pluginsdk.Resource)
	resources := make(map[string]*pluginsdk.Resource)

	// first handle the typed services
	for _, service := range SupportedTypedServices() {
		logEntry("[DEBUG] Registering Data Sources for %q..", service.Name())
		for _, ds := range service.DataSources() {
			key := ds.ResourceType()
			if existing := dataSources[key]; existing != nil {
				panic(fmt.Sprintf("An existing Data Source exists for %q", key))
			}

			wrapper := sdk.NewDataSourceWrapper(ds)
			dataSource, err := wrapper.DataSource()
			if err != nil {
				panic(fmt.Errorf("creating Wrapper for Data Source %q: %+v", key, err))
			}

			dataSources[key] = dataSource
		}

		logEntry("[DEBUG] Registering Resources for %q..", service.Name())
		for _, r := range service.Resources() {
			key := r.ResourceType()
			if existing := resources[key]; existing != nil {
				panic(fmt.Sprintf("An existing Resource exists for %q", key))
			}

			wrapper := sdk.NewResourceWrapper(r)
			resource, err := wrapper.Resource()
			if err != nil {
				panic(fmt.Errorf("creating Wrapper for Resource %q: %+v", key, err))
			}
			resources[key] = resource
		}
	}

	// then handle the untyped services
	for _, service := range SupportedUntypedServices() {
		logEntry("[DEBUG] Registering Data Sources for %q..", service.Name())
		for k, v := range service.SupportedDataSources() {
			if existing := dataSources[k]; existing != nil {
				panic(fmt.Sprintf("An existing Data Source exists for %q", k))
			}

			dataSources[k] = v
		}

		logEntry("[DEBUG] Registering Resources for %q..", service.Name())
		for k, v := range service.SupportedResources() {
			if existing := resources[k]; existing != nil {
				panic(fmt.Sprintf("An existing Resource exists for %q", k))
			}

			resources[k] = v
		}
	}

	p := &schema.Provider{
		Schema: map[string]*pluginsdk.Schema{
			"client_id": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The Client ID which should be used for service principal authentication",
			},

			"client_id_file_path": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The path to a file containing the Client ID which should be used for service principal authentication",
			},

			"tenant_id": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The Tenant ID which should be used. Works with all authentication methods except Managed Identity",
			},

			"environment": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The cloud environment which should be used. Possible values are: `global` (also `public`), `usgovernmentl4` (also `usgovernment`), `usgovernmentl5` (also `dod`), and `china`. Defaults to `global`. Not used and should not be specified when `metadata_host` is specified.",
			},

			"metadata_host": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The Hostname which should be used for the Azure Metadata Service.",
			},

			// Client Certificate specific fields
			"client_certificate": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "Base64 encoded PKCS#12 certificate bundle to use when authenticating as a Service Principal using a Client Certificate",
			},

			"client_certificate_password": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The password to decrypt the Client Certificate. For use when authenticating as a Service Principal using a Client Certificate",
			},

			"client_certificate_path": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The path to the Client Certificate associated with the Service Principal for use when authenticating as a Service Principal using a Client Certificate",
			},

			// Client Secret specific fields
			"client_secret": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The application password to use when authenticating as a Service Principal using a Client Secret",
			},

			"client_secret_file_path": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The path to a file containing the application password to use when authenticating as a Service Principal using a Client Secret",
			},

			// OIDC specific fields
			"use_oidc": {
				Type:        pluginsdk.TypeBool,
				Optional:    true,
				Description: "Allow OpenID Connect to be used for authentication",
			},

			"oidc_token": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The ID token for use when authenticating as a Service Principal using OpenID Connect.",
			},

			"oidc_token_file_path": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The path to a file containing an ID token for use when authenticating as a Service Principal using OpenID Connect.",
			},

			"ado_pipeline_service_connection_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure DevOps Pipeline Service Connection ID.",
			},

			"oidc_request_token": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The bearer token for the request to the OIDC provider. For use when authenticating as a Service Principal using OpenID Connect.",
			},

			"oidc_request_url": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The URL for the OIDC provider from which to request an ID token. For use when authenticating as a Service Principal using OpenID Connect.",
			},

			// Azure AKS Workload Identity fields
			"use_aks_workload_identity": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Allow Azure AKS Workload Identity to be used for Authentication.",
			},

			// CLI authentication specific fields
			"use_cli": {
				Type:        pluginsdk.TypeBool,
				Optional:    true,
				Description: "Allow Azure CLI to be used for Authentication",
			},

			// Managed Identity specific fields
			"use_msi": {
				Type:        pluginsdk.TypeBool,
				Optional:    true,
				Description: "Allow Managed Identity to be used for Authentication",
			},

			"msi_endpoint": {
				Type:        pluginsdk.TypeString,
				Optional:    true,
				Description: "The path to a custom endpoint for Managed Identity - in most circumstances this should be detected automatically",
			},

			// Managed Tracking GUID for User-agent
			"partner_id": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ValidateFunc: validation.Any(validation.IsUUID, validation.StringIsEmpty),
				Description:  "A GUID/UUID that is registered with Microsoft to facilitate partner resource usage attribution",
			},
			"disable_terraform_partner_id": {
				Type:        pluginsdk.TypeBool,
				Optional:    true,
				Description: "Disable the Terraform Partner ID, which is used if a custom `partner_id` isn't specified",
			},
		},

		ResourcesMap:   resources,
		DataSourcesMap: dataSources,
	}

	p.ConfigureContextFunc = providerConfigure(p)

	return p
}

func providerConfigure(p *schema.Provider) schema.ConfigureContextFunc {
	return func(ctx context.Context, d *pluginsdk.ResourceData) (interface{}, pluginsdk.Diagnostics) {

		var certData []byte
		if encodedCert := hasStateOrEnvOrDefault(d, "ARM_CLIENT_CERTIFICATE", "client_certificate", ""); encodedCert != "" {
			var err error
			certData, err = decodeCertificate(encodedCert)
			if err != nil {
				return nil, pluginsdk.DiagFromErr(err)
			}
		}

		idToken, err := getOidcToken(d)
		if err != nil {
			return nil, pluginsdk.DiagFromErr(err)
		}

		clientSecret, err := getClientSecret(d)
		if err != nil {
			return nil, pluginsdk.DiagFromErr(err)
		}

		clientId, err := getClientId(d)
		if err != nil {
			return nil, pluginsdk.DiagFromErr(err)
		}

		tenantId, err := getTenantId(d)
		if err != nil {
			return nil, pluginsdk.DiagFromErr(err)
		}

		var env *environments.Environment

		envName := hasStateOrEnvOrDefault(d, "ARM_ENVIRONMENT", "environment", "global")
		metadataHost := hasStateOrEnvOrDefault(d, "ARM_METADATA_HOSTNAME", "metadata_host", "")

		if metadataHost != "" {
			logEntry("[DEBUG] Configuring cloud environment from Metadata Service at %q", metadataHost)
			if env, err = environments.FromEndpoint(ctx, fmt.Sprintf("https://%s", metadataHost)); err != nil {
				return nil, pluginsdk.DiagFromErr(err)
			}
		} else {
			logEntry("[DEBUG] Configuring built-in cloud environment by name: %q", envName)
			if env, err = environments.FromName(envName); err != nil {
				return nil, pluginsdk.DiagFromErr(err)
			}
		}

		if env.MicrosoftGraph == nil {
			return nil, pluginsdk.DiagErrorf("Microsoft Graph was not configured for the specified environment")
		} else if endpoint, ok := env.MicrosoftGraph.Endpoint(); !ok || *endpoint == "" {
			return nil, pluginsdk.DiagErrorf("Microsoft Graph endpoint could not be determined for the specified environment")
		}

		enableAzureCli := hasStateOrEnvOrDefaultBool(d, "ARM_USE_CLI", "use_cli", true)

		var (
			enableManagedIdentity = hasStateOrEnvOrDefaultBool(d, "ARM_USE_MSI", "use_msi", false)
			enableOidc            = hasStateOrEnvOrDefaultBool(d, "ARM_USE_OIDC", "use_oidc", false) || hasStateOrEnvOrDefaultBool(d, "ARM_USE_AKS_WORKLOAD_IDENTITY", "use_aks_workload_identity", false)
		)

		authConfig := &auth.Credentials{
			Environment: *env,
			ClientID:    *clientId,
			TenantID:    *tenantId,

			ClientCertificateData:     certData,
			ClientCertificatePassword: hasStateOrEnvOrDefault(d, "ARM_CLIENT_CERTIFICATE_PASSWORD", "client_certificate_password", ""),
			ClientCertificatePath:     hasStateOrEnvOrDefault(d, "ARM_CLIENT_CERTIFICATE_PATH", "client_certificate_path", ""),
			ClientSecret:              *clientSecret,

			OIDCAssertionToken:             *idToken,
			OIDCTokenRequestURL:            hasStateOrMultiEnvOrDefault(d, "oidc_request_url", "", "ARM_OIDC_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_URL", "SYSTEM_OIDCREQUESTURI"),
			OIDCTokenRequestToken:          hasStateOrMultiEnvOrDefault(d, "oidc_request_token", "", "ARM_OIDC_REQUEST_TOKEN", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "SYSTEM_ACCESSTOKEN"),
			ADOPipelineServiceConnectionID: hasStateOrMultiEnvOrDefault(d, "ado_pipeline_service_connection_id", "", "ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID", "ARM_OIDC_AZURE_SERVICE_CONNECTION_ID"),

			CustomManagedIdentityEndpoint: hasStateOrEnvOrDefault(d, "ARM_MSI_ENDPOINT", "msi_endpoint", ""),

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
		partnerId := hasStateOrEnvOrDefault(d, "ARM_PARTNER_ID", "partner_id", "")
		if partnerId == "" && !hasStateOrEnvOrDefaultBool(d, "ARM_DISABLE_TERRAFORM_PARTNER_ID", "disable_terraform_partner_id", false) {
			partnerId = terraformPartnerId
		}

		return buildClient(ctx, p, authConfig, partnerId)
	}
}

func buildClient(ctx context.Context, p *schema.Provider, authConfig *auth.Credentials, partnerId string) (*clients.Client, pluginsdk.Diagnostics) {
	clientBuilder := clients.ClientBuilder{
		AuthConfig:       authConfig,
		PartnerID:        partnerId,
		TerraformVersion: p.TerraformVersion,
	}

	stopCtx, ok := schema.StopContext(ctx) //nolint:staticcheck
	if !ok {
		stopCtx = ctx
	}

	client, err := clientBuilder.Build(stopCtx)
	if err != nil {
		return nil, pluginsdk.DiagFromErr(err)
	}

	return client, nil
}
