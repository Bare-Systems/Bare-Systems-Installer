package errors

const (
	ExitOK          = 0
	ExitGeneric     = 1
	ExitUsage       = 2
	ExitConfig      = 3
	ExitPrereq      = 4
	ExitAuth        = 5
	ExitNetwork     = 6
	ExitRuntime     = 7
	ExitHealth      = 8
	ExitUpdate      = 9
	ExitRollback    = 10
	ExitDiagnostics = 11
	ExitPermissions = 12
)

type Code string

const (
	CodeOK          Code = "OK"
	CodeGeneric     Code = "ERR_GENERIC"
	CodeUsage       Code = "ERR_USAGE"
	CodeConfig      Code = "ERR_CONFIG"
	CodePrereq      Code = "ERR_PREREQ"
	CodeAuth        Code = "ERR_AUTH"
	CodeNetwork     Code = "ERR_NETWORK"
	CodeRuntime     Code = "ERR_RUNTIME"
	CodeHealth      Code = "ERR_HEALTH"
	CodeUpdate      Code = "ERR_UPDATE"
	CodeRollback    Code = "ERR_ROLLBACK"
	CodeDiagnostics Code = "ERR_DIAGNOSTICS"
	CodePermissions Code = "ERR_PERMISSIONS"
)

func ExitCodeFor(code Code) int {
	switch code {
	case CodeOK:
		return ExitOK
	case CodeUsage:
		return ExitUsage
	case CodeConfig:
		return ExitConfig
	case CodePrereq:
		return ExitPrereq
	case CodeAuth:
		return ExitAuth
	case CodeNetwork:
		return ExitNetwork
	case CodeRuntime:
		return ExitRuntime
	case CodeHealth:
		return ExitHealth
	case CodeUpdate:
		return ExitUpdate
	case CodeRollback:
		return ExitRollback
	case CodeDiagnostics:
		return ExitDiagnostics
	case CodePermissions:
		return ExitPermissions
	default:
		return ExitGeneric
	}
}

func DescriptionFor(code Code) string {
	switch code {
	case CodeOK:
		return "success"
	case CodeUsage:
		return "invalid CLI arguments or unsupported command usage"
	case CodeConfig:
		return "invalid config, schema, or value"
	case CodePrereq:
		return "missing Docker or system prerequisite"
	case CodeAuth:
		return "enrollment or authentication failure"
	case CodeNetwork:
		return "network or Portal connectivity failure"
	case CodeRuntime:
		return "Compose or runtime failure"
	case CodeHealth:
		return "services started but are unhealthy"
	case CodeUpdate:
		return "update failed"
	case CodeRollback:
		return "rollback failed"
	case CodeDiagnostics:
		return "diagnostics failed"
	case CodePermissions:
		return "filesystem or privilege failure"
	default:
		return "unclassified failure"
	}
}
