// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"cmp"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-provider-azuread/internal/helpers/tf/pluginsdk"
)

// logEntry avoids log entries showing up in test output
func logEntry(f string, v ...interface{}) {
	if os.Getenv("TF_LOG") == "" {
		return
	}

	if os.Getenv("TF_ACC") != "" {
		return
	}

	log.Printf(f, v...)
}

func decodeCertificate(clientCertificate string) ([]byte, error) {
	var pfx []byte
	if clientCertificate != "" {
		out := make([]byte, base64.StdEncoding.DecodedLen(len(clientCertificate)))
		n, err := base64.StdEncoding.Decode(out, []byte(clientCertificate))
		if err != nil {
			return pfx, fmt.Errorf("could not decode client certificate data: %v", err)
		}
		pfx = out[:n]
	}
	return pfx, nil
}

func hasStateOrEnvOrDefault(d *pluginsdk.ResourceData, env, key, def string) string {
	if v, ok := d.GetOk(key); ok {
		return v.(string)
	} else if s, ok := os.LookupEnv(env); ok {
		return s
	}
	return def
}

func hasStateOrEnvOrDefaultBool(d *pluginsdk.ResourceData, env, key string, def bool) bool {
	if v, ok := d.GetOk(key); ok {
		return v.(bool)
	} else if s, ok := os.LookupEnv(env); ok {
		b, _ := strconv.ParseBool(s)
		return b
	}
	return def
}

func hasStateOrMultiEnvOrDefault(d *pluginsdk.ResourceData, key, def string, envs ...string) string {
	if v, ok := d.GetOk(key); ok {
		return v.(string)
	}
	values := make([]string, len(envs))
	for i, v := range envs {
		values[i] = os.Getenv(v)
	}
	return cmp.Or(append(values, def)...)
}

func getOidcToken(d *pluginsdk.ResourceData) (*string, error) {
	idToken := hasStateOrEnvOrDefault(d, "ARM_OIDC_TOKEN", "oidc_token", "")

	if path := hasStateOrEnvOrDefault(d, "ARM_OIDC_TOKEN_FILE_PATH", "oidc_token_file_path", ""); path != "" {
		fileTokenRaw, err := os.ReadFile(path)

		if err != nil {
			return nil, fmt.Errorf("reading OIDC Token from file %q: %v", path, err)
		}

		fileToken := strings.TrimSpace(string(fileTokenRaw))

		if idToken != "" && idToken != fileToken {
			return nil, fmt.Errorf("mismatch between supplied OIDC token and supplied OIDC token file contents - please either remove one or ensure they match")
		}

		idToken = fileToken
	}

	return &idToken, nil
}

func getClientId(d *pluginsdk.ResourceData) (*string, error) {
	clientId := hasStateOrEnvOrDefault(d, "ARM_CLIENT_ID", "client_id", "")

	if path := hasStateOrEnvOrDefault(d, "ARM_CLIENT_ID_FILE_PATH", "client_id_file_path", ""); path != "" {
		fileClientIdRaw, err := os.ReadFile(path)

		if err != nil {
			return nil, fmt.Errorf("reading Client ID from file %q: %v", path, err)
		}

		fileClientId := strings.TrimSpace(string(fileClientIdRaw))

		if clientId != "" && clientId != fileClientId {
			return nil, fmt.Errorf("mismatch between supplied Client ID and supplied Client ID file contents - please either remove one or ensure they match")
		}

		clientId = fileClientId
	}

	return &clientId, nil
}

func getClientSecret(d *pluginsdk.ResourceData) (*string, error) {
	clientSecret := hasStateOrEnvOrDefault(d, "ARM_CLIENT_SECRET", "client_secret", "")

	if path := hasStateOrEnvOrDefault(d, "ARM_CLIENT_SECRET_FILE_PATH", "client_secret_file_path", ""); path != "" {
		fileSecretRaw, err := os.ReadFile(path)

		if err != nil {
			return nil, fmt.Errorf("reading Client Secret from file %q: %v", path, err)
		}

		fileSecret := strings.TrimSpace(string(fileSecretRaw))

		if clientSecret != "" && clientSecret != fileSecret {
			return nil, fmt.Errorf("mismatch between supplied Client Secret and supplied Client Secret file contents - please either remove one or ensure they match")
		}

		clientSecret = fileSecret
	}

	return &clientSecret, nil
}

func getTenantId(d *pluginsdk.ResourceData) (*string, error) {
	tenantId := strings.TrimSpace(hasStateOrEnvOrDefault(d, "ARM_TENANT_ID", "tenant_id", ""))

	if hasStateOrEnvOrDefaultBool(d, "ARM_USE_AKS_WORKLOAD_IDENTITY", "use_aks_workload_identity", false) && os.Getenv("AZURE_TENANT_ID") != "" {
		aksTenantId := os.Getenv("AZURE_TENANT_ID")
		if tenantId != "" && tenantId != aksTenantId {
			return nil, fmt.Errorf("mismatch between supplied Tenant ID and that provided by AKS Workload Identity - please remove, ensure they match, or disable use_aks_workload_identity")
		}
		tenantId = aksTenantId
	}

	return &tenantId, nil
}
