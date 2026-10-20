package translate

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/translator"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Options contains the translate command options
type Options struct {
	InputFile      string
	Target         string
	OutputFile     string
	ResourcePrefix string
	RegistryPrefix string
}

var opts Options

// NewCommand creates the translate command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "translate -f <file> -t <target>",
		Short: "Translate pipeline to different deployment formats",
		Long: `Translate a pipeline specification into different deployment formats
such as Docker Compose or for local execution.

Examples:
  # Translate to Docker Compose and print to stdout
  flowctl translate -f pipeline.yaml -t docker-compose

  # Save to a file
  flowctl translate -f pipeline.yaml -t docker-compose --to-file docker-compose.yml

  # Translate with resource prefix for naming consistency
  flowctl translate -f pipeline.yaml -t docker-compose --prefix myapp

  # Wrap the result in a JSON envelope, for a script or an agent
  flowctl translate -f pipeline.yaml -t docker-compose --output json | jq -r '.data.content'`,
		Args: cobra.NoArgs,
		RunE: runTranslate,
	}

	// Add flags.
	//
	// The deployment target used to be --output/-o, which collided with the
	// global --output selecting an encoding (CLI-3). It is --target/-t now.
	cmd.Flags().StringVarP(&opts.InputFile, "file", "f", "", "input pipeline specification file")
	cmd.Flags().StringVarP(&opts.Target, "target", "t", "", "deployment target (docker-compose, local)")
	cmd.Flags().StringVar(&opts.OutputFile, "to-file", "", "write the result to a file instead of stdout")
	cmd.Flags().StringVar(&opts.ResourcePrefix, "prefix", "", "prefix for resource names")
	cmd.Flags().StringVar(&opts.RegistryPrefix, "registry", "", "container registry prefix")

	return cmd
}

// runTranslate handles the execution of the translate command
func runTranslate(cmd *cobra.Command, args []string) error {
	p := cliout.FromContext(cmd.Context())

	if opts.InputFile == "" {
		return exitcode.Errorf(exitcode.Usage, "input file is required; use -f to specify one")
	}
	if opts.Target == "" {
		return exitcode.Errorf(exitcode.Usage, "deployment target is required; use -t to specify one")
	}

	options := model.TranslationOptions{
		ResourcePrefix: opts.ResourcePrefix,
		RegistryPrefix: opts.RegistryPrefix,
		OutputPath:     opts.OutputFile,
	}
	t := translator.NewTranslator(options)

	format := model.Format(opts.Target)
	validFormats := t.ValidFormats()
	isValid := false
	for _, f := range validFormats {
		if f == format {
			isValid = true
			break
		}
	}
	if !isValid {
		return exitcode.Errorf(exitcode.Usage,
			"invalid target %q; valid targets: %v", format, validFormats)
	}

	logger.Info("Translating pipeline",
		zap.String("input", opts.InputFile),
		zap.String("target", string(format)))

	result, err := t.TranslateFromFile(cmd.Context(), opts.InputFile, format)
	if err != nil {
		// Translation fails when the pipeline is incomplete for the target —
		// a missing image, say. That is a data validation failure, not an
		// unclassified one (CLI-15).
		return exitcode.Wrap(exitcode.DataInvalid, fmt.Errorf("translation failed: %w", err))
	}

	if opts.OutputFile != "" {
		// The translator already wrote the file; report what happened rather
		// than echoing the content.
		return p.Object(cliout.Object{
			Schema: "flowctl.translation.v1",
			Fields: []cliout.Field{
				{Label: "Source", Key: "source"},
				{Label: "Target", Key: "target"},
				{Label: "Written to", Key: "path"},
			},
			Data: cliout.Row{
				"source": opts.InputFile,
				"target": string(format),
				"path":   opts.OutputFile,
			},
		})
	}

	// In a machine format the generated artifact is a field of the envelope, so
	// a caller gets the target alongside the content. In table mode stdout is
	// the artifact itself, unadorned, so `flowctl translate … > compose.yml`
	// keeps working.
	if p.Machine() {
		return p.Object(cliout.Object{
			Schema: "flowctl.translation.v1",
			Data: cliout.Row{
				"source":  opts.InputFile,
				"target":  string(format),
				"content": string(result),
			},
		})
	}

	_, err = fmt.Fprintln(p.Out(), string(result))
	return err
}
