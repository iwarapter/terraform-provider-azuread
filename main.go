// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
	"github.com/hashicorp/terraform-provider-azuread/internal/framework"

	"github.com/hashicorp/terraform-provider-azuread/version"
	"log"

	"github.com/hashicorp/terraform-provider-azuread/internal/provider"
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	p, err := New()
	if err != nil {
		return
	}

	var serveOpts []tf6server.ServeOpt

	if debugMode {
		serveOpts = append(serveOpts, tf6server.WithManagedDebug())
	}

	err = tf6server.Serve("registry.terraform.io/hashicorp/azuread", p, serveOpts...)
	if err != nil {
		log.Fatal(err)
	}
}

func New() (func() tfprotov6.ProviderServer, error) {
	ctx := context.Background()

	// Upgrade the provider sdkv2 version to protocol 6
	upgradedSdkProvider, err := tf5to6server.UpgradeServer(
		context.Background(),
		provider.AzureADProvider().GRPCProvider,
	)
	if err != nil {
		return nil, err
	}

	providers := []func() tfprotov6.ProviderServer{
		func() tfprotov6.ProviderServer {
			return upgradedSdkProvider
		},
		providerserver.NewProtocol6(
			framework.New(version.ProviderVersion),
		),
	}

	muxServer, err := tf6muxserver.NewMuxServer(ctx, providers...)

	if err != nil {
		return nil, err
	}
	return muxServer.ProviderServer, nil
}
