package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/runner"
	"github.com/withobsrvr/flowctl/internal/validator"
)

var validateCmd = &cobra.Command{
	Use:   "validate [pipeline-file]",
	Short: "Validate a pipeline configuration file",
	Long: `Validate a pipeline YAML file before running it.

This command checks for common configuration issues including:
- YAML structure and required fields
- Duplicate component IDs
- Basic pipeline wiring and inputs
- Port conflicts
- Component executability (command, image, or type)
- Processor chain and sink fan-out warnings

Exits 8 when validation fails, 3 when the file does not exist.

Examples:
  # Validate a pipeline file
  flowctl validate my-pipeline.yaml

  # Validate default pipeline
  flowctl validate

  # Gate a deploy on it, reading the reasons from JSON
  flowctl validate pipeline.yaml --output json | jq -r '.data.errors[].message'`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := cliout.FromContext(cmd.Context())

		pipelineFile := "flow.yml"
		if len(args) > 0 {
			pipelineFile = args[0]
		}

		if _, err := os.Stat(pipelineFile); err != nil {
			return exitcode.Errorf(exitcode.NotFound, "pipeline file not found: %s", pipelineFile)
		}

		pipeline, loadErr := runner.LoadPipelineFromFile(pipelineFile)
		if loadErr != nil {
			// A file that will not parse is a validation failure, not a
			// different kind of error. Reporting it through the same envelope
			// means `validate --output json` always emits one document, so a
			// caller never has to handle two shapes.
			if err := emitResult(p, pipelineFile, validationRecord{
				Valid:  false,
				Errors: []map[string]any{{"field": "", "message": loadErr.Error()}},
			}, loadErr.Error()); err != nil {
				return err
			}
			return exitcode.Errorf(exitcode.DataInvalid,
				"%s could not be loaded: %v", pipelineFile, loadErr)
		}

		result := validator.NewValidator(pipeline).Validate()

		rec := validationRecord{Valid: result.Valid, Hints: result.Hints}
		for _, e := range result.Errors {
			item := map[string]any{"field": e.Field, "message": e.Message}
			if e.Fix != "" {
				item["fix"] = e.Fix
			}
			if e.Line > 0 {
				item["line"] = e.Line
			}
			rec.Errors = append(rec.Errors, item)
		}
		for _, w := range result.Warnings {
			item := map[string]any{"field": w.Field, "message": w.Message}
			if w.Hint != "" {
				item["hint"] = w.Hint
			}
			rec.Warnings = append(rec.Warnings, item)
		}

		if err := emitResult(p, pipelineFile, rec, result.Format()); err != nil {
			return err
		}

		if !result.Valid {
			// A failed validation is data invalid, not a generic failure
			// (CLI-15). This used to be a bare os.Exit(1), which bypassed the
			// exit-code contract entirely.
			return exitcode.Errorf(exitcode.DataInvalid,
				"%s failed validation with %d error(s)", pipelineFile, len(result.Errors))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

// validationRecord is the machine-readable shape of a validation outcome.
type validationRecord struct {
	Valid    bool
	Errors   []map[string]any
	Warnings []map[string]any
	Hints    []string
}

// emitResult writes a validation outcome in whichever format was asked for.
// humanText is what a person sees; it is ignored in machine mode.
func emitResult(p *cliout.Printer, file string, rec validationRecord, humanText string) error {
	if !p.Machine() {
		_, err := p.Out().Write([]byte(humanText + "\n"))
		return err
	}
	if rec.Errors == nil {
		rec.Errors = []map[string]any{}
	}
	if rec.Warnings == nil {
		rec.Warnings = []map[string]any{}
	}
	if rec.Hints == nil {
		rec.Hints = []string{}
	}
	return p.Object(cliout.Object{
		Schema: "flowctl.validation_result.v1",
		Data: cliout.Row{
			"file":     file,
			"valid":    rec.Valid,
			"errors":   rec.Errors,
			"warnings": rec.Warnings,
			"hints":    rec.Hints,
		},
	})
}
