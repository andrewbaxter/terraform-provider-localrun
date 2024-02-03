package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type ThisProvider struct {
}

// Configure implements provider.Provider
func (ThisProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

// DataSources implements provider.Provider
func (ThisProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource {
			return DataAlwaysRun{}
		},
	}
}

// Metadata implements provider.Provider
func (ThisProvider) Metadata(_ context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "localrun"
}

// Resources implements provider.Provider
func (ThisProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			return Run{}
		},
	}
}

// Schema implements provider.Provider
func (ThisProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		Description: "Run a local command",
	}
}

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(
		context.Background(),
		func() provider.Provider {
			return ThisProvider{}
		},
		providerserver.ServeOpts{
			Address: "registry.terraform.io/andrewbaxter/localrun",
			Debug:   debug,
		},
	)

	if err != nil {
		log.Fatal(err.Error())
	}
}
