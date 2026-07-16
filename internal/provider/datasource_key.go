package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &keyDataSource{}

type keyDataSource struct {
	client *vaultClient
}

type keyDataSourceModel struct {
	Backend              types.String `tfsdk:"backend"`
	Name                 types.String `tfsdk:"name"`
	Algorithm            types.String `tfsdk:"algorithm"`
	Identity             types.String `tfsdk:"identity"`
	Exportable           types.Bool   `tfsdk:"exportable"`
	DeletionAllowed      types.Bool   `tfsdk:"deletion_allowed"`
	MinDecryptionVersion types.Int64  `tfsdk:"min_decryption_version"`
	LatestVersion        types.Int64  `tfsdk:"latest_version"`
	Fingerprint          types.String `tfsdk:"fingerprint"`
	KeyID                types.String `tfsdk:"key_id"`
	PublicKey            types.String `tfsdk:"public_key"`
}

func NewKeyDataSource() datasource.DataSource {
	return &keyDataSource{}
}

func (d *keyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (d *keyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an OpenPGP key's metadata and armored public key from a gpg secrets engine mount.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "Mount path of the gpg secrets engine.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the key.",
				Required:    true,
			},
			"algorithm":              schema.StringAttribute{Description: "Key algorithm.", Computed: true},
			"identity":               schema.StringAttribute{Description: "OpenPGP User ID.", Computed: true},
			"exportable":             schema.BoolAttribute{Description: "Whether the private key is exportable.", Computed: true},
			"deletion_allowed":       schema.BoolAttribute{Description: "Whether the key may be deleted.", Computed: true},
			"min_decryption_version": schema.Int64Attribute{Description: "Minimum usable version for decryption/verification.", Computed: true},
			"latest_version":         schema.Int64Attribute{Description: "Current (highest) key version.", Computed: true},
			"fingerprint":            schema.StringAttribute{Description: "OpenPGP fingerprint of the latest version.", Computed: true},
			"key_id":                 schema.StringAttribute{Description: "OpenPGP key ID of the latest version.", Computed: true},
			"public_key":             schema.StringAttribute{Description: "Armored public key of the latest version.", Computed: true},
		},
	}
}

func (d *keyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*vaultClient)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *keyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config keyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	backend := strings.Trim(config.Backend.ValueString(), "/")
	name := config.Name.ValueString()

	data, err := d.client.readKey(ctx, backend, name)
	if err != nil {
		resp.Diagnostics.AddError("failed to read gpg key", err.Error())
		return
	}
	if data == nil {
		resp.Diagnostics.AddError("gpg key not found", fmt.Sprintf("no key %q in backend %q", name, backend))
		return
	}

	if s, ok := toString(data["algorithm"]); ok {
		config.Algorithm = types.StringValue(s)
	}
	if s, ok := toString(data["identity"]); ok {
		config.Identity = types.StringValue(s)
	}
	if b, ok := toBool(data["exportable"]); ok {
		config.Exportable = types.BoolValue(b)
	}
	if b, ok := toBool(data["deletion_allowed"]); ok {
		config.DeletionAllowed = types.BoolValue(b)
	}
	if n, ok := toInt64(data["min_decryption_version"]); ok {
		config.MinDecryptionVersion = types.Int64Value(n)
	}
	if n, ok := toInt64(data["latest_version"]); ok {
		config.LatestVersion = types.Int64Value(n)
	}
	if s, ok := toString(data["fingerprint"]); ok {
		config.Fingerprint = types.StringValue(s)
	}
	if s, ok := toString(data["public_key"]); ok {
		config.PublicKey = types.StringValue(s)
	}
	config.KeyID = types.StringValue(keyIDForLatest(data))

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
