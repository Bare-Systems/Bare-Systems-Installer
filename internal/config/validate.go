package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
)

type ValidationResult struct {
	EnabledModules []string `json:"enabledModules"`
	Profiles       []string `json:"profiles"`
	EnvKeys        []string `json:"envKeys"`
}

type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Problems, "; ")
}

func ValidateDeployment(deployment Deployment, env Environment, registry modules.Registry) (ValidationResult, error) {
	var problems []string

	if deployment.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if deployment.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind must be %q", Kind))
	}
	if strings.TrimSpace(deployment.Metadata.Name) == "" {
		problems = append(problems, "metadata.name is required")
	}
	if strings.TrimSpace(deployment.Spec.Channel) == "" {
		problems = append(problems, "spec.channel is required")
	}
	if strings.TrimSpace(deployment.Spec.ProjectName) == "" {
		problems = append(problems, "spec.projectName is required")
	}
	if deployment.Spec.Networking.PublicHTTPPort <= 0 {
		problems = append(problems, "spec.networking.publicHttpPort must be positive")
	}
	if deployment.Spec.Networking.PublicHTTPSPort <= 0 {
		problems = append(problems, "spec.networking.publicHttpsPort must be positive")
	}

	for _, moduleName := range deployment.ModuleNames() {
		if _, ok := registry.Get(moduleName); !ok {
			problems = append(problems, fmt.Sprintf("unknown module %q", moduleName))
		}
	}

	if !deployment.ModuleEnabled("core") {
		problems = append(problems, "core module must always be enabled")
	}

	activeProfiles := map[string]bool{}
	enabledModules := []string{}
	for _, manifest := range registry.All() {
		if manifest.Module.Required || deployment.ModuleEnabled(manifest.Module.ID) {
			enabledModules = append(enabledModules, manifest.Module.ID)
			for _, profile := range manifest.Module.Profiles {
				activeProfiles[profile] = true
			}
			for _, key := range manifest.Module.Config.Required {
				if _, ok := env[key]; !ok {
					problems = append(problems, fmt.Sprintf("enabled module %q requires config value %q", manifest.Module.ID, key))
				}
			}
		}
	}

	knownProfiles := registry.Profiles()
	for _, profile := range deployment.Spec.Runtime.Profiles {
		if !knownProfiles[profile] {
			problems = append(problems, fmt.Sprintf("unknown profile %q", profile))
			continue
		}
		if !activeProfiles[profile] {
			problems = append(problems, fmt.Sprintf("profile %q is not enabled by any active module", profile))
		}
	}

	for _, key := range env.Keys() {
		if SecretLikeKey(key) {
			problems = append(problems, fmt.Sprintf(".env key %q looks secret-like; use /etc/bare-systems/secrets files instead", key))
		}
	}

	sort.Strings(enabledModules)
	profiles := mapKeys(activeProfiles)
	result := ValidationResult{
		EnabledModules: enabledModules,
		Profiles:       profiles,
		EnvKeys:        env.Keys(),
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return result, ValidationError{Problems: problems}
	}
	return result, nil
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
