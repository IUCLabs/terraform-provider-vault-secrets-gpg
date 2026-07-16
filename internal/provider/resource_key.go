package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &keyResource{}
	_ resource.ResourceWithImportState = &keyResource{}
)

type keyResource struct {
	client *vaultClient
}

type keyModel struct {
	Backend              types.String `tfsdk:"backend"`
	Name                 types.String `tfsdk:"name"`
	Algorithm            types.String `tfsdk:"algorithm"`
	Identity             types.String `tfsdk:"identity"`
	Exportable           types.Bool   `tfsdk:"exportable"`
	DeletionAllowed      types.Bool   `tfsdk:"deletion_allowed"`
	MinDecryptionVersion types.Int64  `tfsdk:"min_decryption_version"`

	LatestVersion types.Int64  `tfsdk:"latest_version"`
	Fingerprint   types.String `tfsdk:"fingerprint"`
	KeyID         types.String `tfsdk:"key_id"`
	PublicKey     types.String `tfsdk:"public_key"`
}

func NewKeyResource() resource.Resource {
	return &keyResource{}
}

func (r *keyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (r *keyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Manages an OpenPGP key in a gpg secrets engine mount.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description:   "Mount path of the gpg secrets engine (e.g. \"gpg\").",
				Required:      true,
				PlanModifiers: requiresReplace,
			},
			"name": schema.StringAttribute{
				Description:   "Name of the key.",
				Required:      true,
				PlanModifiers: requiresReplace,
			},
			"algorithm": schema.StringAttribute{
				Description:   "Key algorithm: rsa-2048, rsa-3072, rsa-4096 or ed25519.",
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("rsa-3072"),
				PlanModifiers: requiresReplace,
			},
			"identity": schema.StringAttribute{
				Description:   "OpenPGP User ID (e.g. \"Me <me@example>\"). Defaults to the key name.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: requiresReplace,
			},
			"exportable": schema.BoolAttribute{
				Description: "Allow exporting the private key. Can be enabled later but never disabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"deletion_allowed": schema.BoolAttribute{
				Description: "Whether the key may be deleted. Terraform enables this automatically on destroy.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"min_decryption_version": schema.Int64Attribute{
				Description: "Minimum key version usable for decryption and verification.",
				Optional:    true,
				Computed:    true,
			},
			"latest_version": schema.Int64Attribute{
				Description: "The current (highest) key version.",
				Computed:    true,
			},
			"fingerprint": schema.StringAttribute{
				Description: "OpenPGP fingerprint of the latest version.",
				Computed:    true,
			},
			"key_id": schema.StringAttribute{
				Description: "OpenPGP key ID of the latest version.",
				Computed:    true,
			},
			"public_key": schema.StringAttribute{
				Description: "Armored public key of the latest version (import into gpg/pass to encrypt to this key).",
				Computed:    true,
			},
		},
	}
}

func (r *keyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*vaultClient)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *keyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan keyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	backend := strings.Trim(plan.Backend.ValueString(), "/")
	name := plan.Name.ValueString()

	create := map[string]interface{}{
		"algorithm":  plan.Algorithm.ValueString(),
		"exportable": plan.Exportable.ValueBool(),
	}
	if !plan.Identity.IsNull() && !plan.Identity.IsUnknown() && plan.Identity.ValueString() != "" {
		create["identity"] = plan.Identity.ValueString()
	}
	if _, err := r.client.writeKey(ctx, backend, name, create); err != nil {
		resp.Diagnostics.AddError("failed to create gpg key", err.Error())
		return
	}

	if err := r.applyConfig(ctx, backend, name, plan); err != nil {
		resp.Diagnostics.AddError("failed to configure gpg key", err.Error())
		return
	}

	if diags := r.refresh(ctx, backend, name, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *keyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state keyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	backend := strings.Trim(state.Backend.ValueString(), "/")
	name := state.Name.ValueString()

	data, err := r.client.readKey(ctx, backend, name)
	if err != nil {
		resp.Diagnostics.AddError("failed to read gpg key", err.Error())
		return
	}
	if data == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	applyKeyData(&state, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *keyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan keyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	backend := strings.Trim(plan.Backend.ValueString(), "/")
	name := plan.Name.ValueString()

	if err := r.applyConfig(ctx, backend, name, plan); err != nil {
		resp.Diagnostics.AddError("failed to update gpg key config", err.Error())
		return
	}

	if diags := r.refresh(ctx, backend, name, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *keyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state keyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	backend := strings.Trim(state.Backend.ValueString(), "/")
	name := state.Name.ValueString()

	// Ensure deletion is permitted so `terraform destroy` always succeeds.
	if err := r.client.writeKeyConfig(ctx, backend, name, map[string]interface{}{"deletion_allowed": true}); err != nil {
		resp.Diagnostics.AddError("failed to allow deletion of gpg key", err.Error())
		return
	}
	if err := r.client.deleteKey(ctx, backend, name); err != nil {
		resp.Diagnostics.AddError("failed to delete gpg key", err.Error())
		return
	}
}

// ImportState accepts "<backend>/<name>".
func (r *keyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	backend, name, found := strings.Cut(req.ID, "/")
	if !found || backend == "" || name == "" {
		resp.Diagnostics.AddError("invalid import ID", "expected \"<backend>/<name>\", e.g. \"gpg/app\"")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("backend"), backend)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

// applyConfig pushes the mutable key policy (deletion_allowed / exportable /
// min_decryption_version) via keys/<name>/config.
func (r *keyResource) applyConfig(ctx context.Context, backend, name string, m keyModel) error {
	cfg := map[string]interface{}{}
	if !m.DeletionAllowed.IsNull() && !m.DeletionAllowed.IsUnknown() {
		cfg["deletion_allowed"] = m.DeletionAllowed.ValueBool()
	}
	// exportable can only be turned on, so only send it when true.
	if !m.Exportable.IsNull() && !m.Exportable.IsUnknown() && m.Exportable.ValueBool() {
		cfg["exportable"] = true
	}
	if !m.MinDecryptionVersion.IsNull() && !m.MinDecryptionVersion.IsUnknown() {
		cfg["min_decryption_version"] = m.MinDecryptionVersion.ValueInt64()
	}
	if len(cfg) == 0 {
		return nil
	}
	return r.client.writeKeyConfig(ctx, backend, name, cfg)
}

// refresh re-reads the key and applies it onto the model (computed fields).
func (r *keyResource) refresh(ctx context.Context, backend, name string, m *keyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	data, err := r.client.readKey(ctx, backend, name)
	if err != nil {
		diags.AddError("failed to read gpg key after write", err.Error())
		return diags
	}
	if data == nil {
		diags.AddError("gpg key vanished", fmt.Sprintf("key %q not found in backend %q immediately after write", name, backend))
		return diags
	}
	applyKeyData(m, data)
	return diags
}

// applyKeyData maps an engine key response onto the model.
func applyKeyData(m *keyModel, data map[string]interface{}) {
	if s, ok := toString(data["algorithm"]); ok {
		m.Algorithm = types.StringValue(s)
	}
	if s, ok := toString(data["identity"]); ok {
		m.Identity = types.StringValue(s)
	}
	if b, ok := toBool(data["exportable"]); ok {
		m.Exportable = types.BoolValue(b)
	}
	if b, ok := toBool(data["deletion_allowed"]); ok {
		m.DeletionAllowed = types.BoolValue(b)
	}
	if n, ok := toInt64(data["min_decryption_version"]); ok {
		m.MinDecryptionVersion = types.Int64Value(n)
	}
	if n, ok := toInt64(data["latest_version"]); ok {
		m.LatestVersion = types.Int64Value(n)
	}
	if s, ok := toString(data["fingerprint"]); ok {
		m.Fingerprint = types.StringValue(s)
	}
	if s, ok := toString(data["public_key"]); ok {
		m.PublicKey = types.StringValue(s)
	}
	m.KeyID = types.StringValue(keyIDForLatest(data))
}

// keyIDForLatest digs the latest version's key_id out of the keys map.
func keyIDForLatest(data map[string]interface{}) string {
	latest, ok := toInt64(data["latest_version"])
	if !ok {
		return ""
	}
	versions, ok := data["keys"].(map[string]interface{})
	if !ok {
		return ""
	}
	info, ok := versions[fmt.Sprintf("%d", latest)].(map[string]interface{})
	if !ok {
		return ""
	}
	if s, ok := toString(info["key_id"]); ok {
		return s
	}
	return ""
}
