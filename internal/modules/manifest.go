package modules

import "sort"

const (
	APIVersion = "bare.systems/v1alpha1"
	Kind       = "ModuleManifest"
)

type Manifest struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Module     Module   `json:"module" yaml:"module"`
}

type Metadata struct {
	Name        string `json:"name" yaml:"name"`
	Version     int    `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Module struct {
	ID             string              `json:"id" yaml:"id"`
	Required       bool                `json:"required" yaml:"required"`
	DefaultEnabled bool                `json:"defaultEnabled" yaml:"defaultEnabled"`
	Profiles       []string            `json:"profiles" yaml:"profiles"`
	Images         map[string]ImageRef `json:"images" yaml:"images"`
	Services       []Service           `json:"services" yaml:"services"`
	Config         ConfigContract      `json:"config" yaml:"config"`
	Ports          []string            `json:"ports" yaml:"ports"`
	Volumes        []string            `json:"volumes" yaml:"volumes"`
	Secrets        []Secret            `json:"secrets" yaml:"secrets"`
}

type ImageRef struct {
	Image string `json:"image" yaml:"image"`
}

type Service struct {
	Name            string      `json:"name" yaml:"name"`
	ComposeService  string      `json:"composeService" yaml:"composeService"`
	Image           string      `json:"image" yaml:"image"`
	ImageRepository string      `json:"imageRepository,omitempty" yaml:"imageRepository,omitempty"`
	Profiles        []string    `json:"profiles" yaml:"profiles"`
	Ports           []string    `json:"ports,omitempty" yaml:"ports,omitempty"`
	ExtraHosts      []string    `json:"extraHosts,omitempty" yaml:"extraHosts,omitempty"`
	Volumes         []string    `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Secrets         []string    `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	DependsOn       []string    `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
	Health          HealthCheck `json:"health" yaml:"health"`
}

type HealthCheck struct {
	Type    string   `json:"type" yaml:"type"`
	URL     string   `json:"url,omitempty" yaml:"url,omitempty"`
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
}

type Secret struct {
	Name string `json:"name" yaml:"name"`
	File string `json:"file" yaml:"file"`
}

type ConfigContract struct {
	Required []string `json:"required,omitempty" yaml:"required,omitempty"`
	Optional []string `json:"optional,omitempty" yaml:"optional,omitempty"`
}

type Registry struct {
	manifests map[string]Manifest
	order     []string
}

func NewRegistry(manifests []Manifest) Registry {
	registry := Registry{manifests: map[string]Manifest{}}
	for _, manifest := range manifests {
		registry.manifests[manifest.Module.ID] = manifest
		registry.order = append(registry.order, manifest.Module.ID)
	}
	sort.Strings(registry.order)
	return registry
}

func (r Registry) Get(id string) (Manifest, bool) {
	manifest, ok := r.manifests[id]
	return manifest, ok
}

func (r Registry) All() []Manifest {
	manifests := make([]Manifest, 0, len(r.order))
	for _, id := range r.order {
		manifests = append(manifests, r.manifests[id])
	}
	return manifests
}

func (r Registry) Profiles() map[string]bool {
	profiles := map[string]bool{}
	for _, manifest := range r.manifests {
		for _, profile := range manifest.Module.Profiles {
			profiles[profile] = true
		}
	}
	return profiles
}
