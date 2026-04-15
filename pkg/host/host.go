package host

type Host struct {
	FS      FileSystem
	Exec    Executor
	Network NetworkHost
	System  System
}

func NewHost(fs FileSystem, exec Executor, network NetworkHost, system System) *Host {
	return &Host{
		FS:      fs,
		Exec:    exec,
		Network: network,
		System:  system,
	}
}

func NewDefaultHost() *Host {
	return NewHost(NewOsFS(), &CommandExecutor{}, &NetworkHostImpl{}, NewDefaultSystemHost())
}

func (h *Host) ExecuteIfNotExist(file string, fn func() error) error {
	return ExecuteIfNotExist(h.FS, file, fn)
}

func (h *Host) ExecuteIfExist(file string, fn func() error) error {
	return ExecuteIfExist(h.FS, file, fn)
}
