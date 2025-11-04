package deps

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

//go:embed templates/grid_dependencies.tf.tmpl
var dependenciesTemplate string

var syncLogicID string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync dependencies to HCL file",
	Long: `Fetches all producer states for a consumer and generates a grid_dependencies.tf file
with Terraform data sources for each producer's remote state. This allows the consumer
to reference producer outputs via data.terraform_remote_state.<producer>.outputs.<key>

If --state is not specified, the .grid context will be used (if available).`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		// Resolve state from --state flag or .grid context
		logicID := syncLogicID
		if logicID == "" {
			// Try to read .grid context
			gridCtx, err := dirctx.ReadGridContext()
			if err != nil {
				fmt.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
				return fmt.Errorf("--state flag is required (no .grid context found)")
			} else if gridCtx != nil {
				logicID = gridCtx.StateLogicID
				fmt.Printf("Using state from .grid context: %s\n", logicID)
			} else {
				return fmt.Errorf("--state flag is required (no .grid context found)")
			}
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		graph, err := gridClient.GetDependencyGraph(ctx, sdk.StateReference{LogicID: logicID})
		if err != nil {
			return fmt.Errorf("failed to get dependency graph: %w", err)
		}

		if len(graph.Producers) == 0 {
			fmt.Println("No dependencies found. Nothing to sync.")
			return nil
		}

		tmpl, err := template.New("grid_dependencies").Parse(dependenciesTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		producerSlugByGUID := make(map[string]string)
		slugUsage := make(map[string]int)

		makeUniqueSlug := func(base string) string {
			count := slugUsage[base]
			slugUsage[base] = count + 1
			if count == 0 {
				return base
			}
			return fmt.Sprintf("%s_%d", base, count)
		}

		type producerData struct {
			GUID          string
			LogicID       string
			SafeLogicID   string
			BackendConfig sdk.BackendConfig
		}

		producers := make([]producerData, 0, len(graph.Producers))
		for _, producer := range graph.Producers {
			baseSlug := sanitizeLogicID(producer.State.LogicID)
			uniqueSlug := makeUniqueSlug(baseSlug)
			producerSlugByGUID[producer.State.GUID] = uniqueSlug
			producers = append(producers, producerData{
				GUID:          producer.State.GUID,
				LogicID:       producer.State.LogicID,
				SafeLogicID:   uniqueSlug,
				BackendConfig: producer.BackendConfig,
			})
		}

		type dependencyData struct {
			ToInputName  string
			ProducerSlug string
			FromOutput   string
		}

		dependencies := make([]dependencyData, 0, len(graph.Edges))
		for _, edge := range graph.Edges {
			if strings.EqualFold(edge.Status, "missing-output") {
				continue
			}

			slug := producerSlugByGUID[edge.From.GUID]
			if slug == "" {
				base := sanitizeLogicID(edge.From.LogicID)
				slug = makeUniqueSlug(base)
				producerSlugByGUID[edge.From.GUID] = slug
			}

			inputName := ""
			if edge.ToInputName != "" {
				inputName = edge.ToInputName
			} else {
				inputName = fmt.Sprintf("%s_%s", sanitizeLogicID(edge.From.LogicID), sanitizeLogicID(edge.FromOutput))
			}

			dependencies = append(dependencies, dependencyData{
				ToInputName:  inputName,
				ProducerSlug: slug,
				FromOutput:   edge.FromOutput,
			})
		}

		sort.Slice(producers, func(i, j int) bool { return producers[i].SafeLogicID < producers[j].SafeLogicID })
		sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].ToInputName < dependencies[j].ToInputName })

		var buf bytes.Buffer
		data := struct {
			ConsumerGUID    string
			ConsumerLogicID string
			Producers       []producerData
			Dependencies    []dependencyData
			Timestamp       string
		}{
			ConsumerGUID:    graph.Consumer.GUID,
			ConsumerLogicID: graph.Consumer.LogicID,
			Producers:       producers,
			Dependencies:    dependencies,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		}

		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render template: %w", err)
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine working directory: %w", err)
		}

		filename := filepath.Join(dir, "grid_dependencies.tf")
		tmpFile, err := os.CreateTemp(dir, "grid_dependencies_*.tmp")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
		tmpName := tmpFile.Name()
		defer os.Remove(tmpName)

		if _, err := tmpFile.Write(buf.Bytes()); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write temporary file: %w", err)
		}
		if err := tmpFile.Sync(); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to flush temporary file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file: %w", err)
		}
		if err := os.Chmod(tmpName, 0o644); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}

		if err := os.Rename(tmpName, filename); err != nil {
			if removeErr := os.Remove(filename); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return fmt.Errorf("failed to replace grid_dependencies.tf: %w", err)
			}
			if err := os.Rename(tmpName, filename); err != nil {
				return fmt.Errorf("failed to write grid_dependencies.tf: %w", err)
			}
		}

		fmt.Printf("Generated grid_dependencies.tf with %d dependencies\n", len(dependencies))
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncLogicID, "state", "", "Logic ID of the consumer state (uses .grid context if not specified)")
}

// sanitizeLogicID converts a logic ID to a safe Terraform identifier
// Replaces non-alphanumeric characters with underscores
func sanitizeLogicID(logicID string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	safe := reg.ReplaceAllString(logicID, "_")
	safe = strings.Trim(safe, "_")
	if safe == "" || (safe[0] >= '0' && safe[0] <= '9') {
		safe = "state_" + safe
	}
	return safe
}
