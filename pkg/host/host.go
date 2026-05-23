package host

type FileExecutor interface {
	FileSystem
	Executor
}

type FileEnvironment interface {
	FileSystem
	Environment
}

type UserHost interface {
	FileSystem
	Environment
	Executor
}

type Host interface {
	FileExecutor
	NetworkHost
	System
	Environment
}

type HostProvider interface {
	Host() Host
}

func NewDefaultHost() Host {
	return NewOsFS().(*hostImpl) //nolint:errcheck,forcetypeassert // Good type
}
