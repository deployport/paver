package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/deployport/paver/pkg/projects"
	"github.com/deployport/paver/pkg/templates"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

func main() {
	ctx := context.Background()
	var rootCmd = &cobra.Command{
		Use:   "pave",
		Short: "Pave is a tool for scaffolding Go projects",
		Long: `Pave is a tool for scaffolding Go projects. It uses Go's embed
package to embed the template files into the binary. This allows the tool to
be distributed as a single binary.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(ctx)
		},
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

const defaultGoModName = "github.com/username/generated-proj"

func run(ctx context.Context) error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	defer logger.Sync()

	goModName := question(fmt.Sprintf("What Go module name would you like for your new project? [%s]: ", defaultGoModName))
	if goModName == "" {
		goModName = defaultGoModName
	}

	projectName := path.Base(goModName)
	// wd, err := os.Getwd()
	// if err != nil {
	// 	return err
	// }
	baseDir := "$PWD/" + projectName

	userDir := question(fmt.Sprintf("Where would you like us to create your new project? [%s]: ", baseDir))
	if userDir != "" {
		baseDir = userDir
	}
	baseDir = os.ExpandEnv(baseDir)
	logger.Info("baseDir", zap.String("path", baseDir))

	templatePath, err := templates.DecompressFromURL("https://github.com/deployport/pave-template-pgx-entgo-gqlgen-zap/archive/refs/tags/v0.1.0.tar.gz")
	if err != nil {
		return err
	}
	defer os.RemoveAll(templatePath)
	templatePath, err = templates.GetTempTemplateSubDirectory(templatePath)
	if err != nil {
		return err
	}
	logger.Info("templatePath", zap.String("path", templatePath))
	templateDir := os.DirFS(templatePath)
	gomodContent, err := fs.ReadFile(templateDir, "go.mod")
	if err != nil {
		return fmt.Errorf("failed to read go.mod:, %w", err)
	}
	gomod, err := modfile.Parse("go.mod", gomodContent, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod:, %w", err)
	}

	projectContent, err := fs.ReadFile(templateDir, "pave.yml")
	if err != nil {
		return fmt.Errorf("failed to read pave.yml:, %w", err)
	}
	projectConfig := projects.Config{}
	err = yaml.Unmarshal(projectContent, &projectConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal pave.yml:, %w", err)
	}

	defaultData := map[string]string{
		"GoModName":   goModName,
		"ProjectName": projectName,
	}
	replaceStrings := map[string]string{}
	for k, v := range projectConfig.Rename {
		tmpl := template.New("templated-file-" + k)
		templates.InstallStandardTemplateFunctions(tmpl)
		tmpl, err = tmpl.Parse(v)
		if err != nil {
			return fmt.Errorf("failed to parse template:, %w", err)
		}
		var buf bytes.Buffer
		err = tmpl.Execute(&buf, defaultData)
		if err != nil {
			return fmt.Errorf("failed to execute template for replacement value:, %w", err)
		}
		replaceStrings[k] = buf.String()
	}
	replaceStrings[gomod.Module.Mod.Path] = goModName

	err = templates.NewExpander(templateDir, defaultData,
		replaceStrings,
		logger,
	).Expand(baseDir)
	if err != nil {
		return err
	}
	if err := execute(baseDir, "go", "mod", "tidy"); err != nil {
		return err
	}
	if err := execute(baseDir, "go", "mod", "vendor"); err != nil {
		return err
	}
	if err := execute(baseDir, "go", "generate", "./..."); err != nil {
		return err
	}

	return nil
}

func question(q string) string {
	// Create a new scanner that reads from standard input
	scanner := bufio.NewScanner(os.Stdin)

	// Ask the user for input
	fmt.Print(q)

	// Get the input from the user
	scanner.Scan()

	// Store the user's input
	input := scanner.Text()
	return strings.TrimSpace(input)
}

func execute(baseDir string, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = baseDir

	// Run the command
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
