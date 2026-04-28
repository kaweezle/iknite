package host

type FileExecutor interface {
	FileSystem
	Executor
}

type Host interface {
	FileExecutor
	NetworkHost
	System
}

type HostProvider interface {
	Host() Host
}

func NewDefaultHost() Host {
	return NewOsFS().(*hostImpl) //nolint:errcheck,forcetypeassert // Good type
}
