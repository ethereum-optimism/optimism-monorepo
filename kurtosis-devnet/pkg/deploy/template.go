package deploy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/build"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
)

type Templater struct {
	enclave      string
	dryRun       bool
	baseDir      string
	templateFile string
	dataFile     string
	buildDir     string
	urlBuilder   func(path ...string) string
}

func (f *Templater) localDockerImageOption() tmpl.TemplateContextOptions {
	dockerBuilder := build.NewDockerBuilder(
		build.WithDockerBaseDir(f.baseDir),
		build.WithDockerDryRun(f.dryRun),
	)

	imageTag := func(projectName string) string {
		return fmt.Sprintf("%s:%s", projectName, f.enclave)
	}

	return tmpl.WithFunction("localDockerImage", func(projectName string) (string, error) {
		return dockerBuilder.Build(projectName, imageTag(projectName))
	})
}

func (f *Templater) localContractArtifactsOption() tmpl.TemplateContextOptions {
	contractsBundle := fmt.Sprintf("contracts-bundle-%s.tar.gz", f.enclave)
	contractsBundlePath := func(_ string) string {
		return filepath.Join(f.buildDir, contractsBundle)
	}
	contractsURL := f.urlBuilder(contractsBundle)

	contractBuilder := build.NewContractBuilder(
		build.WithContractBaseDir(f.baseDir),
		build.WithContractDryRun(f.dryRun),
	)

	return tmpl.WithFunction("localContractArtifacts", func(layer string) (string, error) {
		bundlePath := contractsBundlePath(layer)
		if err := contractBuilder.Build(layer, bundlePath); err != nil {
			return "", err
		}

		log.Printf("%s: contract artifacts available at: %s\n", layer, contractsURL)
		return contractsURL, nil
	})
}

func (f *Templater) localPrestateOption() tmpl.TemplateContextOptions {
	holder := &localPrestateHolder{
		baseDir:  f.baseDir,
		buildDir: f.buildDir,
		dryRun:   f.dryRun,
		builder: build.NewPrestateBuilder(
			build.WithPrestateBaseDir(f.baseDir),
			build.WithPrestateDryRun(f.dryRun),
		),
		urlBuilder: f.urlBuilder,
	}

	return tmpl.WithFunction("localPrestate", func() (*PrestateInfo, error) {
		return holder.GetPrestateInfo()
	})
}

func (f *Templater) Render() (*bytes.Buffer, error) {
	opts := []tmpl.TemplateContextOptions{
		f.localDockerImageOption(),
		f.localContractArtifactsOption(),
		f.localPrestateOption(),
		tmpl.WithBaseDir(f.baseDir),
	}

	// Read and parse the data file if provided
	if f.dataFile != "" {
		data, err := os.ReadFile(f.dataFile)
		if err != nil {
			return nil, fmt.Errorf("error reading data file: %w", err)
		}

		var templateData map[string]interface{}
		if err := json.Unmarshal(data, &templateData); err != nil {
			return nil, fmt.Errorf("error parsing JSON data: %w", err)
		}

		opts = append(opts, tmpl.WithData(templateData))
	}

	// Open template file
	tmplFile, err := os.Open(f.templateFile)
	if err != nil {
		return nil, fmt.Errorf("error opening template file: %w", err)
	}
	defer tmplFile.Close()

	// Create template context
	tmplCtx := tmpl.NewTemplateContext(opts...)

	// Process template
	buf := bytes.NewBuffer(nil)
	if err := tmplCtx.InstantiateTemplate(tmplFile, buf); err != nil {
		return nil, fmt.Errorf("error processing template: %w", err)
	}

	return buf, nil
}
