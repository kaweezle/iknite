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

func NewDefaultHost() Host {
	return NewOsFS().(*hostImpl) //nolint:errcheck,forcetypeassert // Good type
}
