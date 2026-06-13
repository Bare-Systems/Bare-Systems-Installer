package modules

const defaultSecretsDir = "/etc/bare-systems/secrets"

func BuiltInRegistry() Registry {
	return NewRegistry([]Manifest{
		coreManifest(),
		moduleManifest("koala", "Camera and home security services", "KOALA_SITE_ID"),
		moduleManifest("polar", "Operational monitoring services", "POLAR_SITE_ID"),
		moduleManifest("kodiak", "Local orchestration services", "KODIAK_SITE_ID"),
		moduleManifest("ursa", "Security scanning services", "URSA_SITE_ID"),
	})
}

func coreManifest() Manifest {
	return Manifest{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Name:        "core",
			Version:     1,
			Description: "Required reverse proxy, web UI, and AI runtime integration",
		},
		Module: Module{
			ID:             "core",
			Required:       true,
			DefaultEnabled: true,
			Profiles:       []string{"core"},
			Images: map[string]ImageRef{
				"tardigrade":     {Image: "${TARDIGRADE_IMAGE:-registry.example.com/bare/tardigrade:unspecified}"},
				"bearclaw-web":   {Image: "${BEARCLAW_WEB_IMAGE:-registry.example.com/bare/bearclaw-web:unspecified}"},
				"bearclaw-agent": {Image: "${BEARCLAW_AGENT_IMAGE:-registry.example.com/bare/bearclaw-agent:unspecified}"},
			},
			Services: []Service{
				{
					Name:           "tardigrade",
					ComposeService: "tardigrade",
					Image:          "${TARDIGRADE_IMAGE:-registry.example.com/bare/tardigrade:unspecified}",
					Profiles:       []string{"core"},
					Ports:          []string{"${PUBLIC_HTTP_PORT:-80}:80", "${PUBLIC_HTTPS_PORT:-443}:443"},
					Secrets:        []string{"tls-cert", "tls-key"},
					Health:         HealthCheck{Type: "http", URL: "http://localhost/health"},
				},
				{
					Name:           "bearclaw-web",
					ComposeService: "bearclaw-web",
					Image:          "${BEARCLAW_WEB_IMAGE:-registry.example.com/bare/bearclaw-web:unspecified}",
					Profiles:       []string{"core"},
					Health:         HealthCheck{Type: "http", URL: "http://localhost:8080/health"},
				},
				{
					Name:           "bearclaw-agent",
					ComposeService: "bearclaw-agent",
					Image:          "${BEARCLAW_AGENT_IMAGE:-registry.example.com/bare/bearclaw-agent:unspecified}",
					Profiles:       []string{"core"},
					Secrets:        []string{"portal-token"},
					Health:         HealthCheck{Type: "exec", Command: []string{"CMD", "/app/healthcheck"}},
				},
			},
			Config: ConfigContract{
				Required: []string{"BARE_CHANNEL", "BARE_PROJECT_NAME", "PUBLIC_HTTP_PORT", "PUBLIC_HTTPS_PORT"},
				Optional: []string{"ADMIN_BIND_ADDRESS"},
			},
			Ports:   []string{"80", "443"},
			Volumes: []string{"bare-state"},
			Secrets: []Secret{
				{Name: "portal-token", File: defaultSecretsDir + "/portal-token"},
				{Name: "tls-cert", File: defaultSecretsDir + "/tls-cert"},
				{Name: "tls-key", File: defaultSecretsDir + "/tls-key"},
			},
		},
	}
}

func moduleManifest(id string, description string, requiredConfig string) Manifest {
	serviceName := id + "-agent"
	image := "${" + upperEnv(id) + "_IMAGE:-registry.example.com/bare/" + serviceName + ":unspecified}"
	return Manifest{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Name:        id,
			Version:     1,
			Description: description,
		},
		Module: Module{
			ID:             id,
			Required:       false,
			DefaultEnabled: false,
			Profiles:       []string{id},
			Images: map[string]ImageRef{
				serviceName: {Image: image},
			},
			Services: []Service{
				{
					Name:           serviceName,
					ComposeService: serviceName,
					Image:          image,
					Profiles:       []string{id},
					Volumes:        []string{id + "-data:/var/lib/bare-systems/" + id},
					Health:         HealthCheck{Type: "exec", Command: []string{"CMD", "/app/healthcheck"}},
				},
			},
			Config: ConfigContract{
				Required: []string{requiredConfig},
				Optional: []string{upperEnv(id) + "_RETENTION_DAYS"},
			},
			Volumes: []string{id + "-data"},
			Secrets: []Secret{},
		},
	}
}

func upperEnv(id string) string {
	switch id {
	case "koala":
		return "KOALA"
	case "polar":
		return "POLAR"
	case "kodiak":
		return "KODIAK"
	case "ursa":
		return "URSA"
	default:
		return "BARE"
	}
}
