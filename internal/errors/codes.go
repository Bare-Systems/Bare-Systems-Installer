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
