package config

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "bare.systems/v1alpha1"
	Kind       = "EdgeDeployment"
)

type Deployment struct {
	APIVersion string         `json:"apiVersion" yaml:"apiVersion"`
	Kind       string         `json:"kind" yaml:"kind"`
	Metadata   Metadata       `json:"metadata" yaml:"metadata"`
	Spec       DeploymentSpec `json:"spec" yaml:"spec"`
}

type Metadata struct {
	Name        string `json:"name" yaml:"name"`
	Customer    string `json:"customer,omitempty" yaml:"customer,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type DeploymentSpec struct {
	Channel     string                  `json:"channel" yaml:"channel"`
	ProjectName string                  `json:"projectName" yaml:"projectName"`
	Runtime     RuntimeSpec             `json:"runtime" yaml:"runtime"`
	Modules     map[string]ModuleConfig `json:"modules" yaml:"modules"`
	Networking  NetworkingSpec          `json:"networking" yaml:"networking"`
	Storage     StorageSpec             `json:"storage" yaml:"storage"`
}

type RuntimeSpec struct {
	ComposeProjectDirectory string   `json:"composeProjectDirectory,omitempty" yaml:"composeProjectDirectory,omitempty"`
	DockerContext           string   `json:"dockerContext,omitempty" yaml:"dockerContext,omitempty"`
	Profiles                []string `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}

type ModuleConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type NetworkingSpec struct {
	PublicHTTPPort   int    `json:"publicHttpPort" yaml:"publicHttpPort"`
	PublicHTTPSPort  int    `json:"publicHttpsPort" yaml:"publicHttpsPort"`
	AdminBindAddress string `json:"adminBindAddress" yaml:"adminBindAddress"`
}

type StorageSpec struct {
	Root    string `json:"root" yaml:"root"`
	Backups string `json:"backups,omitempty" yaml:"backups,omitempty"`
}

type LoadResult struct {
	Deployment Deployment `json:"deployment"`
	Source     string     `json:"source"`
	Warnings   []string   `json:"warnings"`
}

func DefaultDeployment() Deployment {
	return Deployment{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Name: "local-edge",
		},
		Spec: DeploymentSpec{
			Channel:     "stable",
			ProjectName: "bare-systems",
			Runtime: RuntimeSpec{
				ComposeProjectDirectory: DefaultComposeDir,
				DockerContext:           "default",
				Profiles:                []string{"core"},
			},
			Modules: map[string]ModuleConfig{
				"core":   {Enabled: true},
				"koala":  {Enabled: false},
				"polar":  {Enabled: false},
				"kodiak": {Enabled: false},
				"ursa":   {Enabled: false},
			},
			Networking: NetworkingSpec{
				PublicHTTPPort:   80,
				PublicHTTPSPort:  443,
				AdminBindAddress: "0.0.0.0",
			},
			Storage: StorageSpec{
				Root:    DefaultProjectDir,
				Backups: DefaultProjectDir + "/backups",
			},
		},
	}
}

func LoadDeployment(path string, allowDefault bool) (LoadResult, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if allowDefault && os.IsNotExist(err) {
			return LoadResult{
				Deployment: DefaultDeployment(),
				Source:     "built-in-default",
				Warnings:   []string{fmt.Sprintf("%s not found; using built-in default deployment", path)},
			}, nil
		}
		return LoadResult{}, fmt.Errorf("read deployment config %s: %w", path, err)
	}

	deployment, err := DecodeDeployment(data)
	if err != nil {
		return LoadResult{}, fmt.Errorf("parse deployment config %s: %w", path, err)
	}
	return LoadResult{Deployment: deployment, Source: path}, nil
}

func DecodeDeployment(data []byte) (Deployment, error) {
	var deployment Deployment
	decoder := yaml.NewDecoder(bytesReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&deployment); err != nil {
		return Deployment{}, err
	}
	return deployment, nil
}

func DefaultDeploymentYAML() ([]byte, error) {
	return DeploymentYAML(DefaultDeployment())
}

func DeploymentYAML(deployment Deployment) ([]byte, error) {
	data, err := yaml.Marshal(deployment)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (d Deployment) ModuleEnabled(id string) bool {
	if id == "core" {
		if module, ok := d.Spec.Modules[id]; ok {
			return module.Enabled
		}
		return true
	}
	module, ok := d.Spec.Modules[id]
	return ok && module.Enabled
}

func (d Deployment) ModuleNames() []string {
	names := make([]string, 0, len(d.Spec.Modules))
	for name := range d.Spec.Modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (d Deployment) ComposeProjectName() string {
	if d.Spec.ProjectName != "" {
		return d.Spec.ProjectName
	}
	return "bare-systems"
}

func (d Deployment) ComposeProjectDirectory() string {
	if d.Spec.Runtime.ComposeProjectDirectory != "" {
		return d.Spec.Runtime.ComposeProjectDirectory
	}
	return DefaultComposeDir
}
