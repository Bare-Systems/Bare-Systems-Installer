package config

const (
	DefaultConfigPath = "/etc/bare-systems/edge.yml"
	DefaultProjectDir = "/opt/bare-systems"
	DefaultLogDir     = "/var/log/bare-systems"
)

type Paths struct {
	ConfigPath string
	ProjectDir string
	LogDir     string
}

func DefaultPaths() Paths {
	return Paths{
		ConfigPath: DefaultConfigPath,
		ProjectDir: DefaultProjectDir,
		LogDir:     DefaultLogDir,
	}
}
