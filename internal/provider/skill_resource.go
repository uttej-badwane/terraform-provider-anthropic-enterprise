package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewSkillResource) }

var (
	_ resource.Resource                = &skillResource{}
	_ resource.ResourceWithConfigure   = &skillResource{}
	_ resource.ResourceWithImportState = &skillResource{}
	_ resource.ResourceWithModifyPlan  = &skillResource{}
)

// NewSkillResource returns the anthropic_skill resource.
func NewSkillResource() resource.Resource { return &skillResource{} }

type skillResource struct{ client *client.Client }

type skillModel struct {
	ID              types.String `tfsdk:"id"`
	SourceDir       types.String `tfsdk:"source_dir"`
	DisplayName     types.String `tfsdk:"display_name"`
	ContentHash     types.String `tfsdk:"content_hash"`
	LatestVersionID types.String `tfsdk:"latest_version_id"`
	Name            types.String `tfsdk:"name"`
	SourceType      types.String `tfsdk:"source_type"`
	DeleteOnDestroy types.Bool   `tfsdk:"delete_on_destroy"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

const (
	skillMaxFiles = 20
	skillMaxBytes = 30 << 20
)

func (r *skillResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (r *skillResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Uploads a custom skill from a local directory and keeps it current. The directory must contain a " +
			"`SKILL.md` with `name` and `description` frontmatter; its basename becomes the skill's top-level directory. The provider " +
			"hashes the files on every plan: when the content changes it uploads a new skill version and `latest_version_id` moves. " +
			"Skills have no update endpoint and the API returns no content hash, so changes made outside Terraform are not detected.\n\n" +
			"`terraform destroy` deletes the skill and every version. Set `delete_on_destroy = false` to leave it in place. " +
			"Deleting a version that an agent or deployment pins breaks them at run time." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Skill id (`skill_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_dir": schema.StringAttribute{
				MarkdownDescription: "Local directory holding `SKILL.md` and up to 20 files (30 MB total). Changing the path forces a new skill.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display label, up to 255 characters. Defaults to the `SKILL.md` name. Cannot be changed after creation.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(255)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()},
			},
			"content_hash": schema.StringAttribute{
				MarkdownDescription: "SHA-256 over the sorted file paths and contents of `source_dir`, computed locally on every plan.",
				Computed:            true,
			},
			"latest_version_id": schema.StringAttribute{
				MarkdownDescription: "Id of the newest version; pin agents to it with `skills[].version`.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Slug from the `SKILL.md` frontmatter; every version must keep it.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_type": schema.StringAttribute{
				MarkdownDescription: "Always `custom` for uploaded skills.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Delete the skill (all versions) on destroy. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp (RFC 3339).", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last update timestamp (RFC 3339).", Computed: true},
		},
	}
}

func (r *skillResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

// ModifyPlan computes content_hash from the local files so content changes
// surface as an in-place update.
func (r *skillResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var dir types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("source_dir"), &dir)...)
	if resp.Diagnostics.HasError() || dir.IsUnknown() || dir.IsNull() {
		return
	}
	files, hash, err := readSkillDir(dir.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Invalid skill directory", err.Error())
		return
	}
	_ = files
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("content_hash"), types.StringValue(hash))...)
	var stateHash types.String
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("content_hash"), &stateHash)...)
		if !stateHash.IsNull() && stateHash.ValueString() != hash {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("latest_version_id"), types.StringUnknown())...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("updated_at"), types.StringUnknown())...)
		}
	}
}

// readSkillDir loads every regular file under dir (relative paths prefixed
// with the directory basename) and returns a stable content hash.
func readSkillDir(dir string) ([]client.SkillFile, string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, "", err
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("%s is not a directory", dir)
	}
	top := filepath.Base(filepath.Clean(dir))
	var files []client.SkillFile
	var total int64
	hasSkillMD := false
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		total += int64(len(data))
		if rel == "SKILL.md" {
			hasSkillMD = true
		}
		files = append(files, client.SkillFile{Path: top + "/" + filepath.ToSlash(rel), Data: data})
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if !hasSkillMD {
		return nil, "", fmt.Errorf("%s must contain SKILL.md at its root", dir)
	}
	if len(files) > skillMaxFiles {
		return nil, "", fmt.Errorf("skill has %d files; the API accepts at most %d", len(files), skillMaxFiles)
	}
	if total > skillMaxBytes {
		return nil, "", fmt.Errorf("skill is %d bytes; the API accepts at most %d", total, skillMaxBytes)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%d\x00", f.Path, len(f.Data))
		h.Write(f.Data)
	}
	return files, hex.EncodeToString(h.Sum(nil)), nil
}

func (r *skillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan skillModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	files, hash, err := readSkillDir(plan.SourceDir.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Invalid skill directory", err.Error())
		return
	}
	sk, err := r.client.CreateSkill(ctx, stringPtr(plan.DisplayName), files)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating skill", err)
		return
	}
	tflog.Trace(ctx, "created skill", map[string]any{"id": sk.ID})
	state := plan
	state.ContentHash = types.StringValue(hash)
	resp.Diagnostics.Append(r.flatten(ctx, sk, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *skillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state skillModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sk, err := r.client.GetSkill(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading skill", err)
		return
	}
	resp.Diagnostics.Append(r.flatten(ctx, sk, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *skillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state skillModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	files, hash, err := readSkillDir(plan.SourceDir.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Invalid skill directory", err.Error())
		return
	}
	if hash != state.ContentHash.ValueString() {
		if _, err := r.client.CreateSkillVersion(ctx, state.ID.ValueString(), files); err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error uploading skill version", err)
			return
		}
	}
	sk, err := r.client.GetSkill(ctx, state.ID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading skill", err)
		return
	}
	newState := plan
	newState.ContentHash = types.StringValue(hash)
	resp.Diagnostics.Append(r.flatten(ctx, sk, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *skillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state skillModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.DeleteOnDestroy.ValueBool() {
		tflog.Info(ctx, "delete_on_destroy is false; leaving skill in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if err := r.client.DeleteSkill(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting skill", err)
	}
}

func (r *skillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("delete_on_destroy"), types.BoolValue(true))...)
}

func (r *skillResource) flatten(ctx context.Context, sk *client.Skill, m *skillModel) (diags diagList) {
	m.ID = types.StringValue(sk.ID)
	m.DisplayName = types.StringValue(sk.DisplayName)
	m.LatestVersionID = types.StringValue(sk.LatestVersionID)
	m.SourceType = types.StringValue(sk.Source.Type)
	m.CreatedAt = types.StringValue(sk.CreatedAt)
	m.UpdatedAt = types.StringValue(sk.UpdatedAt)
	if m.DeleteOnDestroy.IsNull() || m.DeleteOnDestroy.IsUnknown() {
		m.DeleteOnDestroy = types.BoolValue(true)
	}
	if m.ContentHash.IsUnknown() {
		m.ContentHash = types.StringNull()
	}
	if v, err := r.client.GetSkillVersion(ctx, sk.ID, "latest"); err == nil {
		m.Name = types.StringValue(v.Name)
	} else {
		diags.AddError("Error reading latest skill version", err.Error())
	}
	return diags
}
