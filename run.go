package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/armon/circbuf"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/mitchellh/go-linereader"
)

func run(ctx context.Context, command []types.String, environment types.Map, workingDir types.String, diagnostics *diag.Diagnostics) {
	cmd0 := []string{}
	for _, c := range command {
		cmd0 = append(cmd0, c.ValueString())
	}
	cmd := exec.Command(cmd0[0], cmd0[1:]...)
	env := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		env[parts[0]] = parts[1]
	}
	for k, v := range environment.Elements() {
		v1 := v.(basetypes.StringValue)
		env[k] = v1.ValueString()
	}
	env1 := []string{}
	for k, v := range env {
		env1 = append(env1, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env1
	cmd.Dir = workingDir.ValueString()

	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		diagnostics.AddError("Failed to initialize pipe for output", err.Error())
		return
	}
	cmd.Stderr = pipeWrite
	cmd.Stdout = pipeWrite

	output, _ := circbuf.NewBuffer(10 * 1024 * 1024)

	copyDoneCh := make(chan struct{})
	go func() {
		defer close(copyDoneCh)
		lr := linereader.New(io.TeeReader(pipeRead, output))
		for line := range lr.Ch {
			tflog.Info(ctx, line)
		}
	}()

	err = cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}
	pipeWrite.Close()

	select {
	case <-copyDoneCh:
	case <-ctx.Done():
	}

	if err != nil {
		diagnostics.AddError("Run failed", fmt.Sprintf("%s\nStdout/err:\n%s", err, string(output.Bytes())))
		return
	}
}

func updateHashes(workingDir types.String, outputs []types.String, outputHashes *types.List, diagnostics *diag.Diagnostics) {
	out := []attr.Value{}
	for _, o := range outputs {
		var p string
		if filepath.IsAbs(o.ValueString()) {
			// Work around filepath incorrectly joining abs to abs paths
			p = o.ValueString()
		} else {
			var err error
			p, err = filepath.Abs(filepath.Join(workingDir.ValueString(), o.ValueString()))
			if err != nil {
				panic(err)
			}
		}
		digest := sha256.New()
		f, err := os.Open(p)
		if err != nil {
			diagnostics.AddError("Failed to open output file "+p, err.Error())
			goto Err
		}
		defer func() {
			err := f.Close()
			if err != nil {
				diagnostics.AddWarning("Failed to close output file "+p, err.Error())
			}
		}()
		_, err = io.Copy(digest, f)
		if err != nil {
			diagnostics.AddError("Failed to hash output file "+p, err.Error())
			goto Err
		}
		out = append(out, types.StringValue(hex.EncodeToString(digest.Sum([]byte{}))))
		continue
	Err:
		out = append(out, types.StringValue("error"))
	}
	var diags diag.Diagnostics
	*outputHashes, diags = types.ListValue(types.StringType, out)
	diagnostics.Append(diags...)
	if diagnostics.HasError() {
		return
	}
}

type RunModel struct {
	Command      []types.String `tfsdk:"command"`
	Environment  types.Map      `tfsdk:"environment"`
	WorkingDir   types.String   `tfsdk:"working_dir"`
	Outputs      []types.String `tfsdk:"outputs"`
	OutputHashes types.List     `tfsdk:"output_hashes"`
}

type Run struct {
}

func (Run) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state RunModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	run(ctx, state.Command, state.Environment, state.WorkingDir, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updateHashes(state.WorkingDir, state.Outputs, &state.OutputHashes, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (Run) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (Run) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (Run) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run"
}

func (Run) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Runs a command when any attribute changes",
		Attributes: map[string]schema.Attribute{
			"command": schema.ListAttribute{
				MarkdownDescription: "Command to run; first element is executable, remaining elements are arguments",
				ElementType:         types.StringType,
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"environment": schema.MapAttribute{
				MarkdownDescription: "Environment variables; inherits terraform's environment",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"working_dir": schema.StringAttribute{
				MarkdownDescription: "Working directory in which to run command; defaults to current directory",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"outputs": schema.ListAttribute{
				MarkdownDescription: "Paths to files generated by the command, relative to working directory; the hashes of these are placed in `output_hashes` after execution",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"output_hashes": schema.ListAttribute{
				MarkdownDescription: "The hashes of the output files, updated after execution",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r Run) run(ctx context.Context, state *RunModel, diagnostics *diag.Diagnostics) {
}

func (r Run) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

type DataAlwaysRunModel struct {
	Command      []types.String `tfsdk:"command"`
	Environment  types.Map      `tfsdk:"environment"`
	WorkingDir   types.String   `tfsdk:"working_dir"`
	Outputs      []types.String `tfsdk:"outputs"`
	OutputHashes types.List     `tfsdk:"output_hashes"`
}

type DataAlwaysRun struct {
}

func (DataAlwaysRun) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_always_run"
}

func (DataAlwaysRun) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		MarkdownDescription: "Unconditionally run a command (during read phase). This is for actions with dependencies not tracked by Terraform, like a Makefile.",
		Attributes: map[string]datasourceschema.Attribute{
			"command": datasourceschema.ListAttribute{
				MarkdownDescription: "Command to run; first element is executable, remaining elements are arguments",
				ElementType:         types.StringType,
				Required:            true,
			},
			"environment": datasourceschema.MapAttribute{
				MarkdownDescription: "Environment variables; inherits terraform's environment",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"working_dir": datasourceschema.StringAttribute{
				MarkdownDescription: "Working directory in which to run command; defaults to current directory",
				Optional:            true,
			},
			"outputs": datasourceschema.ListAttribute{
				MarkdownDescription: "Paths to files generated by the command, relative to working directory; the hashes of these are placed in `output_hashes` after execution",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"output_hashes": datasourceschema.ListAttribute{
				MarkdownDescription: "The hashes of the output files, updated after execution",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r DataAlwaysRun) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataAlwaysRunModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	run(ctx, state.Command, state.Environment, state.WorkingDir, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updateHashes(state.WorkingDir, state.Outputs, &state.OutputHashes, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
