package config

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Environment map[string]string

type EnvLoadResult struct {
	Environment Environment `json:"environment"`
	Source      string      `json:"source"`
	Warnings    []string    `json:"warnings"`
}

func LoadEnv(path string, explicit bool) (EnvLoadResult, error) {
	if path == "" {
		path = DefaultEnvPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !explicit && os.IsNotExist(err) {
			return EnvLoadResult{
				Environment: Environment{},
				Source:      "not-found",
				Warnings:    []string{fmt.Sprintf("%s not found; using values derived from edge.yml only", path)},
			}, nil
		}
		return EnvLoadResult{}, fmt.Errorf("read env file %s: %w", path, err)
	}

	env, err := ParseEnv(data)
	if err != nil {
		return EnvLoadResult{}, fmt.Errorf("parse env file %s: %w", path, err)
	}
	return EnvLoadResult{Environment: env, Source: path}, nil
}

func ParseEnv(data []byte) (Environment, error) {
	env := Environment{}
	scanner := bufio.NewScanner(bytesReader(data))
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", line)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: key is empty", line)
		}
		env[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return env, nil
}

func DerivedEnv(deployment Deployment) Environment {
	env := Environment{
		"BARE_CHANNEL":        deployment.Spec.Channel,
		"BARE_IMAGE_REGISTRY": "registry.example.com/bare",
		"BARE_IMAGE_TAG":      "unspecified",
		"BARE_PROJECT_NAME":   deployment.ComposeProjectName(),
		"ADMIN_BIND_ADDRESS":  deployment.Spec.Networking.AdminBindAddress,
		"PUBLIC_HTTP_PORT":    strconv.Itoa(deployment.Spec.Networking.PublicHTTPPort),
		"PUBLIC_HTTPS_PORT":   strconv.Itoa(deployment.Spec.Networking.PublicHTTPSPort),
		"BARE_COMPOSE_DIR":    deployment.ComposeProjectDirectory(),
		"BARE_STORAGE_ROOT":   deployment.Spec.Storage.Root,
	}
	return env
}

func MergeEnv(base Environment, override Environment) Environment {
	merged := Environment{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func (e Environment) Keys() []string {
	keys := make([]string, 0, len(e))
	for key := range e {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func SecretLikeKey(key string) bool {
	upper := strings.ToUpper(key)
	secretMarkers := []string{"SECRET", "TOKEN", "PASSWORD", "PRIVATE_KEY", "TLS_KEY", "API_KEY"}
	for _, marker := range secretMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}
