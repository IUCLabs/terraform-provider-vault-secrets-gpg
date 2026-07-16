package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretBackendResource{}
	_ resource.ResourceWithImportState = &secretBackendResource{}
)

const defaultPluginType = "vault-plugin-secrets-gpg"

type secretBackendResource struct {
	client *vaultClient
}

type secretBackendModel struct {
	Path        types.String `tfsdk:"path"`
	Plugin      types.String `tfsdk:"plugin"`
	Description types.String `tfsdk:"description"`
	SHA256      types.String `tfsdk:"sha256"`
	Command     types.String `tfsdk:"command"`
}

func NewSecretBackendResource() resource.Resource {
	return &secretBackendResource{}
}

func (r *secretBackendResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend"
}

func (r *secretBackendResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Mounts the gpg secrets engine. Optionally registers the plugin in the catalog first when a sha256 is given.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Description: "Mount path for the gpg secrets engine (e.g. \"gpg\").",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plugin": schema.StringAttribute{
				Description: "Registered plugin name/type to mount.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(defaultPluginType),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Human-readable description of the mount.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"sha256": schema.StringAttribute{
				Description: "SHA-256 of the plugin binary. When set, the plugin is (re)registered in the catalog before mounting; omit if the plugin is already registered out of band.",
				Optional:    true,
			},
			"command": schema.StringAttribute{
				Description: "Plugin binary filename (relative to the server plugin_directory) used when registering. Defaults to the plugin name. Only used when sha256 is set.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *secretBackendResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// managesPlugin reports whether the resource owns the catalog entry (sha256 set).
func managesPlugin(m secretBackendModel) bool {
	return !m.SHA256.IsNull() && m.SHA256.ValueString() != ""
}

// resolveCommand returns the binary filename to register: explicit command, else
// the plugin name.
func resolveCommand(m secretBackendModel) string {
	if !m.Command.IsNull() && !m.Command.IsUnknown() && m.Command.ValueString() != "" {
		return m.Command.ValueString()
	}
	return m.Plugin.ValueString()
}

func (r *secretBackendResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretBackendModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(plan.Path.ValueString(), "/")
	command := resolveCommand(plan)

	if managesPlugin(plan) {
		if err := r.client.registerPlugin(ctx, plan.Plugin.ValueString(), command, plan.SHA256.ValueString()); err != nil {
			resp.Diagnostics.AddError("failed to register gpg plugin", err.Error())
			return
		}
	}

	if err := r.client.enableMount(ctx, mountPath, plan.Plugin.ValueString(), plan.Description.ValueString()); err != nil {
		if isMountAlreadyExists(err) {
			resp.Diagnostics.AddError(
				"mount path already in use",
				fmt.Sprintf("A secrets engine is already mounted at %q. Import it or choose another path.", mountPath),
			)
			return
		}
		resp.Diagnostics.AddError("failed to enable gpg secrets engine", err.Error())
		return
	}

	plan.Path = types.StringValue(mountPath)
	plan.Command = types.StringValue(command)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretBackendModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")
	mount, err := r.client.mountInfo(ctx, mountPath)
	if err != nil {
		resp.Diagnostics.AddError("failed to read mount", err.Error())
		return
	}
	if mount == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Description = types.StringValue(mount.Description)
	if mount.Type != "" {
		state.Plugin = types.StringValue(mount.Type)
	}

	if managesPlugin(state) {
		info, err := r.client.pluginInfo(ctx, state.Plugin.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("failed to read plugin catalog entry", err.Error())
			return
		}
		if info != nil {
			if info.SHA256 != "" {
				state.SHA256 = types.StringValue(info.SHA256)
			}
			if info.Command != "" {
				state.Command = types.StringValue(info.Command)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *secretBackendResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state secretBackendModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")

	if !plan.Description.Equal(state.Description) {
		if err := r.client.tuneMount(ctx, mountPath, plan.Description.ValueString()); err != nil {
			resp.Diagnostics.AddError("failed to tune mount description", err.Error())
			return
		}
	}

	command := resolveCommand(plan)
	if managesPlugin(plan) && (!plan.SHA256.Equal(state.SHA256) || command != state.Command.ValueString()) {
		if err := r.client.registerPlugin(ctx, plan.Plugin.ValueString(), command, plan.SHA256.ValueString()); err != nil {
			resp.Diagnostics.AddError("failed to re-register gpg plugin", err.Error())
			return
		}
	}

	plan.Path = types.StringValue(mountPath)
	plan.Command = types.StringValue(command)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretBackendModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")
	if err := r.client.disableMount(ctx, mountPath); err != nil {
		resp.Diagnostics.AddError("failed to disable gpg secrets engine", err.Error())
		return
	}

	if managesPlugin(state) {
		// Best effort: the catalog entry may be shared; ignore failures.
		_ = r.client.deregisterPlugin(ctx, state.Plugin.ValueString())
	}
}

func (r *secretBackendResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("path"), req, resp)
}
