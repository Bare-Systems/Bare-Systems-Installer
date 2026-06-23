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
	Image       string               `yaml:"image"`
	Profiles    []string             `yaml:"profiles,omitempty"`
	Ports       []string             `yaml:"ports,omitempty"`
	ExtraHosts  []string             `yaml:"extra_hosts,omitempty"`
	Volumes     []string             `yaml:"volumes,omitempty"`
	Secrets     []string             `yaml:"secrets,omitempty"`
	DependsOn   map[string]DependsOn `yaml:"depends_on,omitempty"`
	Environment map[string]string    `yaml:"environment,omitempty"`
	Healthcheck *Healthcheck         `yaml:"healthcheck,omitempty"`
}

type DependsOn struct {
	Condition string `yaml:"condition,omitempty"`
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
		if !manifest.Module.Required && deployment.ModuleExternal(manifest.Module.ID) {
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
	environment := selectedEnv(env)
	if service.ComposeService == "bear-claw-web" {
		environment["SECRET_KEY_BASE_DUMMY"] = "1"
		environment["DATABASE_URL"] = "postgres://bare@bear-claw-db:5432/bearclaw_production"
	}
	if service.ComposeService == "bear-claw-db" {
		environment = map[string]string{
			"POSTGRES_DB":               "bearclaw_production",
			"POSTGRES_USER":             "bare",
			"POSTGRES_HOST_AUTH_METHOD": "trust",
		}
	}
	return Service{
		Image:       resolveImage(service, env),
		Profiles:    sortedCopy(service.Profiles),
		Ports:       resolveTemplates(sortedCopy(service.Ports), env),
		ExtraHosts:  resolveTemplates(sortedCopy(service.ExtraHosts), env),
		Volumes:     sortedCopy(service.Volumes),
		Secrets:     sortedCopy(service.Secrets),
		DependsOn:   renderDependsOn(service.DependsOn),
		Environment: environment,
		Healthcheck: renderHealthcheck(service.Health),
	}
}

func renderDependsOn(dependsOn []string) map[string]DependsOn {
	if len(dependsOn) == 0 {
		return nil
	}
	rendered := map[string]DependsOn{}
	for _, service := range sortedCopy(dependsOn) {
		rendered[service] = DependsOn{Condition: "service_healthy"}
	}
	return rendered
}

func resolveImage(service modules.Service, env config.Environment) string {
	for _, imageKey := range imageEnvKeys(service) {
		if image := strings.TrimSpace(env[imageKey]); image != "" {
			return image
		}
	}
	if service.ImageRepository == "" && imageTemplateEnvKey(service.Image) == "" {
		return resolveTemplate(service.Image, env)
	}
	registry := strings.TrimRight(strings.TrimSpace(env["BARE_IMAGE_REGISTRY"]), "/")
	tag := strings.TrimSpace(env["BARE_IMAGE_TAG"])
	if registry != "" && tag != "" {
		return registry + "/" + imageRepository(service) + ":" + tag
	}
	return resolveTemplate(service.Image, env)
}

func imageEnvKeys(service modules.Service) []string {
	keys := []string{}
	if key := imageTemplateEnvKey(service.Image); key != "" {
		keys = append(keys, key)
	}
	keys = append(keys, imageEnvKey(service.ComposeService))
	return uniqueStrings(keys)
}

func imageTemplateEnvKey(image string) string {
	parts := composeVariablePattern.FindStringSubmatch(image)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func imageEnvKey(serviceName string) string {
	key := strings.NewReplacer("-", "_", ".", "_").Replace(serviceName)
	return strings.ToUpper(key) + "_IMAGE"
}

func imageRepository(service modules.Service) string {
	if repository := strings.TrimSpace(service.ImageRepository); repository != "" {
		return repository
	}
	return service.ComposeService
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
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
			Test:     []string{"CMD-SHELL", "curl -fsS " + health.URL + " >/dev/null"},
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
	keys := []string{
		"ADMIN_BIND_ADDRESS",
		"BARE_CHANNEL",
		"BARE_PROJECT_NAME",
		"BEARCLAW_LLM_BASE_URL",
		"BEARCLAW_LLM_MODEL",
		"BEARCLAW_LLM_PROVIDER",
		"BEARCLAW_URL",
		"KOALA_URL",
		"KODIAK_URL",
		"POLAR_URL",
		"URSA_URL",
	}
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
