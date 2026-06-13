package config

const (
	DefaultConfigPath  = "/etc/bare-systems/edge.yml"
	DefaultEnvPath     = "/etc/bare-systems/.env"
	DefaultSecretsDir  = "/etc/bare-systems/secrets"
	DefaultProjectDir  = "/opt/bare-systems"
	DefaultComposeDir  = "/opt/bare-systems/compose"
	DefaultManifestDir = "/opt/bare-systems/manifests"
	DefaultStateDir    = "/opt/bare-systems/state"
	DefaultBundleDir   = "/opt/bare-systems/bundles"
	DefaultLogDir      = "/var/log/bare-systems"
)

type Paths struct {
	ConfigPath  string
	EnvPath     string
	SecretsDir  string
	ProjectDir  string
	ComposeDir  string
	ManifestDir string
	StateDir    string
	BundleDir   string
	LogDir      string
}

func DefaultPaths() Paths {
	return Paths{
		ConfigPath:  DefaultConfigPath,
		EnvPath:     DefaultEnvPath,
		SecretsDir:  DefaultSecretsDir,
		ProjectDir:  DefaultProjectDir,
		ComposeDir:  DefaultComposeDir,
		ManifestDir: DefaultManifestDir,
		StateDir:    DefaultStateDir,
		BundleDir:   DefaultBundleDir,
		LogDir:      DefaultLogDir,
	}
}
