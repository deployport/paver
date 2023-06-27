package templates

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"go.uber.org/zap"
)

// Expander is a type that expands templates
type Expander struct {
	fs              fs.FS
	defaultData     map[string]string
	replacedStrings map[string]string
	logger          *zap.Logger
}

// NewExpander creates a new Expander
func NewExpander(
	fs fs.FS,
	defaultData map[string]string,
	replacedStrings map[string]string,
	logger *zap.Logger,
) *Expander {
	return &Expander{
		fs:              fs,
		defaultData:     defaultData,
		replacedStrings: replacedStrings,
		logger:          logger,
	}
}

// Expand expands the templates
func (e *Expander) Expand(outputDir string) error {
	return e.expandDir(outputDir, ".")
}

func (e *Expander) expandDir(outputDir, fsDirPath string) error {
	logger := e.logger.Named("expandDir").With(zap.String("outputDir", outputDir), zap.String("fsDirPath", fsDirPath))
	// Get the template files from the embedded file system
	templateContent, err := fs.ReadDir(e.fs, fsDirPath)
	if err != nil {
		return fmt.Errorf("failed to read template directory:, %w", err)
	}
	logger.Debug("read", zap.Int("count", len(templateContent)))

	// Ensure the base directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory:, %w", err)
	}
	logger.Debug("created base directory")

	for _, file := range templateContent {
		filename := file.Name()
		fsFilePath := filepath.Join(fsDirPath, filename)
		filenameCurated := expandNameTemplate(filename, e.replacedStrings)
		outputFilePath := filepath.Join(outputDir, filenameCurated)
		if file.IsDir() {
			if err := e.expandDir(outputFilePath, fsFilePath); err != nil {
				return fmt.Errorf("failed to expand directory:, %w", err)
			}
			continue
		}
		logger := logger.With(zap.String("fsFilePath", fsFilePath), zap.String("outputFilePath", outputFilePath))
		logger.Debug("expanding file")

		fcontentStat, err := fs.Stat(e.fs, fsFilePath)
		if err != nil {
			return fmt.Errorf("failed to stat file:, %w", err)
		}
		fcontent, err := fs.ReadFile(e.fs, fsFilePath)
		tmpl := template.New("templated-file")
		InstallStandardTemplateFunctions(tmpl)
		tmpl, err = tmpl.Parse(string(fcontent))
		if err != nil {
			return fmt.Errorf("failed to parse template:, %w", err)
		}

		data := e.defaultData

		finalOutputFilePath := outputFilePath
		if strings.HasSuffix(finalOutputFilePath, templateExtension) {
			finalOutputFilePath = strings.TrimSuffix(finalOutputFilePath, templateExtension)
		}

		// Create the output file
		outputFile, err := os.Create(finalOutputFilePath)
		if err != nil {
			return fmt.Errorf("failed to create output file:, %w", err)
		}
		defer outputFile.Close()
		if err := outputFile.Chmod(fcontentStat.Mode()); err != nil {
			return fmt.Errorf("failed to chmod output file:, %w", err)
		}

		buf := &bytes.Buffer{}

		// Execute the template and write to the output file
		if err := tmpl.Execute(buf, data); err != nil {
			return fmt.Errorf("failed to execute template:, %w", err)
		}
		bufString := buf.String()
		for k, v := range e.replacedStrings {
			bufString = strings.ReplaceAll(bufString, k, v)
		}
		if _, err := outputFile.Write([]byte(bufString)); err != nil {
			return fmt.Errorf("failed to write output file:, %w", err)
		}
	}
	return nil
}

func replaceString(s string, replacedStrings map[string]string) string {
	for k, v := range replacedStrings {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

const templateExtension = ".template"

func expandNameTemplate(name string, replacedStrings map[string]string) string {
	return replaceString(name, replacedStrings)
}
