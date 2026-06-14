package diagnostics

import (
	"regexp"
	"strings"
)

const Redacted = "[REDACTED]"

var assignmentSecretPattern = regexp.MustCompile(`(?i)(token|secret|password|private[_-]?key|api[_-]?key|tls[_-]?key)([A-Z0-9_./ -]*)([:=]\s*)([^,\s"']+)`)

func RedactString(value string) string {
	redacted := assignmentSecretPattern.ReplaceAllString(value, "${1}${2}${3}"+Redacted)
	lines := strings.Split(redacted, "\n")
	for i, line := range lines {
		if secretLine(line) {
			key, _, ok := strings.Cut(line, "=")
			if ok {
				lines[i] = key + "=" + Redacted
				continue
			}
			key, _, ok = strings.Cut(line, ":")
			if ok {
				lines[i] = key + ": " + Redacted
			}
		}
	}
	return strings.Join(lines, "\n")
}

func secretLine(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"token", "secret", "password", "private_key", "private-key", "api_key", "api-key", "tls_key", "tls-key"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func RedactBytes(data []byte) []byte {
	return []byte(RedactString(string(data)))
}
