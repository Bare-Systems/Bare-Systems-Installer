package compose

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	"gopkg.in/yaml.v3"
)

type Model struct {
	Name     string             `yaml:"name"`
	Services map[string]Service `yaml:"services"`
	Volumes  map[string]Volume  `yaml:"volumes,omitempty"`
	Secrets  map[string]Secret  `yaml:"secrets,omitempty"`
}

type Service struct {
	Image       string            `yaml:"image"`
	Profiles    []string          `yaml:"profiles,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Secrets     []string          `yaml:"secrets,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Healthcheck *Healthcheck      `yaml:"healthcheck,omitempty"`
}

type Healthcheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Retries  int      `yaml:"retries,omitempty"`
}

type Volume struct{}

type Secret struct {
	File string `yaml:"file"`
}

func Render(deployment config.Deployment, registry modules.Registry, envOverride ...config.Environment) ([]byte, error) {
	model, err := BuildModel(deployment, registry, envOverride...)
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("marshal compose model: %w", err)
	}
	return data, nil
}

func BuildModel(deployment config.Deployment, registry modules.Registry, envOverride ...config.Environment) (Model, error) {
	model := Model{
		Name:     deployment.ComposeProjectName(),
		Services: map[string]Service{},
		Volumes:  map[string]Volume{},
		Secrets:  map[string]Secret{},
	}

	env := config.DerivedEnv(deployment)
	if len(envOverride) > 0 && envOverride[0] != nil {
		env = envOverride[0]
	}
	for _, manifest := range registry.All() {
		if !manifest.Module.Required && !deployment.ModuleEnabled(manifest.Module.ID) {
			continue
		}
		for _, volume := range manifest.Module.Volumes {
			model.Volumes[volume] = Volume{}
		}
		for _, secret := range manifest.Module.Secrets {
			model.Secrets[secret.Name] = Secret{File: secret.File}
		}
		for _, service := range manifest.Module.Services {
			model.Services[service.ComposeService] = renderService(service, env)
		}
	}

	if len(model.Volumes) == 0 {
		model.Volumes = nil
	}
	if len(model.Secrets) == 0 {
		model.Secrets = nil
	}
	return model, nil
}

func ValidateRendered(data []byte) error {
	var decoded struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("rendered Compose YAML is invalid: %w", err)
	}
	if len(decoded.Services) == 0 {
		return fmt.Errorf("rendered Compose YAML has no services")
	}
	return nil
}

var composeVariablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

func renderService(service modules.Service, env config.Environment) Service {
	return Service{
		Image:       resolveImage(service, env),
		Profiles:    sortedCopy(service.Profiles),
		Ports:       resolveTemplates(sortedCopy(service.Ports), env),
		Volumes:     sortedCopy(service.Volumes),
		Secrets:     sortedCopy(service.Secrets),
		Environment: selectedEnv(env),
		Healthcheck: renderHealthcheck(service.Health),
	}
}

func resolveImage(service modules.Service, env config.Environment) string {
	imageKey := imageEnvKey(service.ComposeService)
	if image := strings.TrimSpace(env[imageKey]); image != "" {
		return image
	}
	registry := strings.TrimRight(strings.TrimSpace(env["BARE_IMAGE_REGISTRY"]), "/")
	tag := strings.TrimSpace(env["BARE_IMAGE_TAG"])
	if registry != "" && tag != "" {
		return registry + "/" + service.ComposeService + ":" + tag
	}
	return resolveTemplate(service.Image, env)
}

func imageEnvKey(serviceName string) string {
	key := strings.NewReplacer("-", "_", ".", "_").Replace(serviceName)
	return strings.ToUpper(key) + "_IMAGE"
}

func resolveTemplates(values []string, env config.Environment) []string {
	if len(values) == 0 {
		return nil
	}
	resolved := make([]string, 0, len(values))
	for _, value := range values {
		resolved = append(resolved, resolveTemplate(value, env))
	}
	return resolved
}

func resolveTemplate(value string, env config.Environment) string {
	return composeVariablePattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := composeVariablePattern.FindStringSubmatch(match)
		if len(parts) == 0 {
			return match
		}
		if envValue := strings.TrimSpace(env[parts[1]]); envValue != "" {
			return envValue
		}
		if len(parts) >= 4 {
			return parts[3]
		}
		return ""
	})
}

func renderHealthcheck(health modules.HealthCheck) *Healthcheck {
	switch health.Type {
	case "http":
		return &Healthcheck{
			Test:     []string{"CMD-SHELL", "wget -qO- " + health.URL + " >/dev/null"},
			Interval: "30s",
			Timeout:  "5s",
			Retries:  3,
		}
	case "exec":
		if len(health.Command) == 0 {
			return nil
		}
		return &Healthcheck{
			Test:     health.Command,
			Interval: "30s",
			Timeout:  "5s",
			Retries:  3,
		}
	default:
		return nil
	}
}

func selectedEnv(env config.Environment) map[string]string {
	keys := []string{"BARE_CHANNEL", "BARE_PROJECT_NAME", "ADMIN_BIND_ADDRESS"}
	selected := map[string]string{}
	for _, key := range keys {
		if value, ok := env[key]; ok && value != "" {
			selected[key] = value
		}
	}
	return selected
}

func sortedCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}
