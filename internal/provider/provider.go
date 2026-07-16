package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &gpgProvider{}

type gpgProvider struct {
	version string
}

type gpgProviderModel struct {
	Address types.String `tfsdk:"address"`
	Token   types.String `tfsdk:"token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &gpgProvider{version: version}
	}
}

func (p *gpgProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	// The provider's source address is git.unkin.net/unkin/gpgvaultsecret, but
	// its resources are prefixed "gpg_" (declare it in required_providers under
	// the local name "gpg"), mirroring how google-beta ships google_*.
	resp.TypeName = "gpg"
	resp.Version = p.version
}

func (p *gpgProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage the vault-plugin-secrets-gpg engine — mounts and OpenPGP keys — on HashiCorp Vault or OpenBao.",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				Description: "Address of the Vault/OpenBao server. Falls back to the VAULT_ADDR environment variable.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "Token used to authenticate to Vault/OpenBao. Falls back to the VAULT_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *gpgProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config gpgProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	address := os.Getenv("VAULT_ADDR")
	if !config.Address.IsNull() && config.Address.ValueString() != "" {
		address = config.Address.ValueString()
	}

	token := os.Getenv("VAULT_TOKEN")
	if !config.Token.IsNull() && config.Token.ValueString() != "" {
		token = config.Token.ValueString()
	}

	if address == "" {
		resp.Diagnostics.AddError(
			"missing Vault address",
			"Set the provider \"address\" attribute or the VAULT_ADDR environment variable.",
		)
		return
	}

	client, err := newVaultClient(address, token)
	if err != nil {
		resp.Diagnostics.AddError("failed to create Vault client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *gpgProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSecretBackendResource,
		NewKeyResource,
	}
}

func (p *gpgProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewKeyDataSource,
	}
}
