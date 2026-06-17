package modules

func BuiltInRegistry() Registry {
	return NewRegistry([]Manifest{
		coreManifest(),
		koalaManifest(),
		singleServiceModuleManifest("polar", "Operational monitoring services", "POLAR_SITE_ID", "POLAR_RETENTION_DAYS", "polar", "POLAR_IMAGE"),
		singleServiceModuleManifest("kodiak", "Local orchestration services", "KODIAK_SITE_ID", "KODIAK_RETENTION_DAYS", "kodiak-agent", "KODIAK_IMAGE"),
		singleServiceModuleManifest("ursa", "Security scanning services", "URSA_SITE_ID", "URSA_RETENTION_DAYS", "ursa", "URSA_IMAGE"),
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
				"bear-claw-web": {Image: "${BEARCLAW_WEB_IMAGE:-ghcr.io/bare-systems/bear-claw-web:latest}"},
			},
			Services: []Service{
				{
					Name:            "bear-claw-web",
					ComposeService:  "bear-claw-web",
					Image:           "${BEARCLAW_WEB_IMAGE:-ghcr.io/bare-systems/bear-claw-web:latest}",
					ImageRepository: "bear-claw-web",
					Profiles:        []string{"core"},
					Health:          HealthCheck{Type: "http", URL: "http://localhost:8080/health"},
				},
			},
			Config: ConfigContract{
				Required: []string{"BARE_CHANNEL", "BARE_PROJECT_NAME", "PUBLIC_HTTP_PORT", "PUBLIC_HTTPS_PORT"},
				Optional: []string{"ADMIN_BIND_ADDRESS"},
			},
			Ports:   []string{"80", "443"},
			Secrets: []Secret{},
		},
	}
}

func koalaManifest() Manifest {
	return Manifest{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Name:        "koala",
			Version:     1,
			Description: "Camera and home security services",
		},
		Module: Module{
			ID:             "koala",
			Required:       false,
			DefaultEnabled: false,
			Profiles:       []string{"koala"},
			Images: map[string]ImageRef{
				"koala-orchestrator": {Image: "${KOALA_ORCHESTRATOR_IMAGE:-ghcr.io/bare-systems/koala-orchestrator:latest}"},
				"koala-worker":       {Image: "${KOALA_WORKER_IMAGE:-ghcr.io/bare-systems/koala-worker:latest}"},
			},
			Services: []Service{
				{
					Name:            "koala-orchestrator",
					ComposeService:  "koala-orchestrator",
					Image:           "${KOALA_ORCHESTRATOR_IMAGE:-ghcr.io/bare-systems/koala-orchestrator:latest}",
					ImageRepository: "koala-orchestrator",
					Profiles:        []string{"koala"},
					Volumes:         []string{"koala-data:/var/lib/bare-systems/koala"},
					Health:          HealthCheck{Type: "exec", Command: []string{"CMD", "/app/healthcheck"}},
				},
				{
					Name:            "koala-worker",
					ComposeService:  "koala-worker",
					Image:           "${KOALA_WORKER_IMAGE:-ghcr.io/bare-systems/koala-worker:latest}",
					ImageRepository: "koala-worker",
					Profiles:        []string{"koala"},
					Volumes:         []string{"koala-data:/var/lib/bare-systems/koala"},
					Health:          HealthCheck{Type: "exec", Command: []string{"CMD", "/app/healthcheck"}},
				},
			},
			Config: ConfigContract{
				Required: []string{"KOALA_SITE_ID"},
				Optional: []string{"KOALA_RETENTION_DAYS"},
			},
			Volumes: []string{"koala-data"},
			Secrets: []Secret{},
		},
	}
}

func singleServiceModuleManifest(id string, description string, requiredConfig string, optionalConfig string, serviceName string, imageOverrideKey string) Manifest {
	image := "${" + imageOverrideKey + ":-ghcr.io/bare-systems/" + serviceName + ":latest}"
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
					Name:            serviceName,
					ComposeService:  serviceName,
					Image:           image,
					ImageRepository: serviceName,
					Profiles:        []string{id},
					Volumes:         []string{id + "-data:/var/lib/bare-systems/" + id},
					Health:          HealthCheck{Type: "exec", Command: []string{"CMD", "/app/healthcheck"}},
				},
			},
			Config: ConfigContract{
				Required: []string{requiredConfig},
				Optional: []string{optionalConfig},
			},
			Volumes: []string{id + "-data"},
			Secrets: []Secret{},
		},
	}
}
