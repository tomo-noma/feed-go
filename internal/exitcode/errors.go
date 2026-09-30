package exitcode

type ExitCode int

const (
	CodeInvalidArgs  ExitCode = 1
	CodeNetworkError ExitCode = 2
	CodeTimeout      ExitCode = 3
	CodeParseError   ExitCode = 4
)
