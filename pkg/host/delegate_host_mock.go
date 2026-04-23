// cSpell: words wrapcheck contextcheck
//
//nolint:wrapcheck,errcheck // Returning underlying errors
package host

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bitfield/script"
	"github.com/spf13/afero"
	"github.com/txn2/txeh"
)

type DelegateHost struct {
	Fs   FileSystem
	Exec Executor
	Sys  System
	Net  NetworkHost
}

// Abs implements [Host].
func (d *DelegateHost) Abs(path string) (string, error) {
	return d.Fs.Abs(path)
}

// CheckIpExists implements [Host].
func (d *DelegateHost) CheckIpExists(ip net.IP) (bool, error) {
	return d.Net.CheckIpExists(ip)
}

// Create implements [Host].
func (d *DelegateHost) Create(path string) (afero.File, error) {
	return d.Fs.Create(path)
}

// DirExists implements [Host].
func (d *DelegateHost) DirExists(path string) (bool, error) {
	return d.Fs.DirExists(path)
}

// EvalSymlinks implements [Host].
func (d *DelegateHost) EvalSymlinks(path string) (string, error) {
	return d.Fs.EvalSymlinks(path)
}

// ExecForEach implements [Host].
func (d *DelegateHost) ExecForEach(stdin *script.Pipe, cmd string) *script.Pipe {
	return d.Exec.ExecForEach(stdin, cmd)
}

// ExecPipe implements [Host].
func (d *DelegateHost) ExecPipe(stdin *script.Pipe, cmd string) *script.Pipe {
	return d.Exec.ExecPipe(stdin, cmd)
}

// Exists implements [Host].
func (d *DelegateHost) Exists(path string) (bool, error) {
	return d.Fs.Exists(path)
}

// FindProcess implements [Host].
func (d *DelegateHost) FindProcess(pid int) (Process, error) {
	return d.Exec.FindProcess(pid)
}

// GetHostsConfig implements [Host].
func (d *DelegateHost) GetHostsConfig() *txeh.HostsConfig {
	return d.Net.GetHostsConfig()
}

// GetOutboundIP implements [Host].
func (d *DelegateHost) GetOutboundIP() (net.IP, error) {
	return d.Net.GetOutboundIP()
}

// Glob implements [Host].
func (d *DelegateHost) Glob(pattern string) ([]string, error) {
	return d.Fs.Glob(pattern)
}

// IsHostMapped implements [Host].
func (d *DelegateHost) IsHostMapped(ctx context.Context, ip net.IP, domainName string) (bool, []net.IP) {
	return d.Net.IsHostMapped(ctx, ip, domainName)
}

// MkdirAll implements [Host].
func (d *DelegateHost) MkdirAll(path string, perm os.FileMode) error {
	return d.Fs.MkdirAll(path, perm)
}

// Open implements [Host].
func (d *DelegateHost) Open(path string) (afero.File, error) {
	return d.Fs.Open(path)
}

// OpenFile implements [Host].
func (d *DelegateHost) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	return d.Fs.OpenFile(path, flag, perm)
}

// Pipe implements [Host].
func (d *DelegateHost) Pipe(path string) *script.Pipe {
	return d.Fs.Pipe(path)
}

// PipeRun implements [Host].
func (d *DelegateHost) PipeRun(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error) {
	return d.Exec.PipeRun(stdin, combined, cmd, arguments...)
}

// ReadDir implements [Host].
func (d *DelegateHost) ReadDir(dirname string) ([]os.FileInfo, error) {
	return d.Fs.ReadDir(dirname)
}

// ReadFile implements [Host].
func (d *DelegateHost) ReadFile(path string) ([]byte, error) {
	return d.Fs.ReadFile(path)
}

// Remove implements [Host].
func (d *DelegateHost) Remove(path string) error {
	return d.Fs.Remove(path)
}

// RemoveAll implements [Host].
func (d *DelegateHost) RemoveAll(path string) error {
	return d.Fs.RemoveAll(path)
}

// Rename implements [Host].
func (d *DelegateHost) Rename(oldPath, newPath string) error {
	return d.Fs.Rename(oldPath, newPath)
}

// Run implements [Host].
func (d *DelegateHost) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {
	return d.Exec.Run(combined, cmd, arguments...)
}

// StartCommand implements [Host].
func (d *DelegateHost) StartCommand(ctx context.Context, options *CommandOptions) (Process, error) {
	return d.Exec.StartCommand(ctx, options)
}

// Stat implements [Host].
func (d *DelegateHost) Stat(path string) (os.FileInfo, error) {
	return d.Fs.Stat(path)
}

// Symlink implements [Host].
func (d *DelegateHost) Symlink(oldName, newName string) error {
	return d.Fs.Symlink(oldName, newName)
}

// Unmount implements [Host].
func (d *DelegateHost) Unmount(path string) error {
	return d.Sys.Unmount(path)
}

// Walk implements [Host].
func (d *DelegateHost) Walk(root string, walkFn filepath.WalkFunc) error {
	return d.Fs.Walk(root, walkFn)
}

// WriteFile implements [Host].
func (d *DelegateHost) WriteFile(path string, data []byte, perm os.FileMode) error {
	return d.Fs.WriteFile(path, data, perm)
}

// WritePipe implements [Host].
func (d *DelegateHost) WritePipe(path string, pipe *script.Pipe, flag int, perm os.FileMode) (int64, error) {
	return d.Fs.WritePipe(path, pipe, flag, perm)
}

var _ Host = (*DelegateHost)(nil)

func NewDelegateHost(fs FileSystem, exec Executor, sys System, n NetworkHost) *DelegateHost {
	return &DelegateHost{
		Fs:   fs,
		Exec: exec,
		Sys:  sys,
		Net:  n,
	}
}

var _ NetworkHost = (*DummyNetworkHost)(nil)

type DummyNetworkHost struct {
	mappedHosts map[string][]string
	hostConfig  *txeh.HostsConfig
	ipAddresses []net.IP
}

func (d *DummyNetworkHost) GetOutboundIP() (net.IP, error) {
	if len(d.ipAddresses) == 0 {
		return nil, errors.New("no IP addresses available")
	}
	return d.ipAddresses[0], nil
}

func (d *DummyNetworkHost) CheckIpExists(ip net.IP) (bool, error) {
	for _, existingIP := range d.ipAddresses {
		if existingIP.Equal(ip) {
			return true, nil
		}
	}
	return false, nil
}

func (d *DummyNetworkHost) GetHostsConfig() *txeh.HostsConfig {
	return d.hostConfig
}

func (d *DummyNetworkHost) IsHostMapped(_ context.Context, ip net.IP, domainName string) (bool, []net.IP) {
	ipString := ip.String()
	var mappedIPs []net.IP
	found := false

	for ip, domainNames := range d.mappedHosts {
		for _, mappedName := range domainNames {
			if mappedName == domainName {
				mappedIPs = append(mappedIPs, net.ParseIP(ip))
				if ip == ipString {
					found = true
				}
				// break the inner loop to avoid adding the same IP multiple times
				break
			}
		}
	}
	return found, mappedIPs
}

func (d *DummyNetworkHost) AddIpAddress(ip net.IP) {
	exists, _ := d.CheckIpExists(ip)
	if !exists {
		d.ipAddresses = append(d.ipAddresses, ip)
	}
}

func (d *DummyNetworkHost) MapHost(domainName string, ip net.IP) {
	ipString := ip.String()
	// Check if the domain name is already mapped to the IP
	if slices.Contains(d.mappedHosts[ipString], domainName) {
		return // Domain name is already mapped to this IP, no need to add it again
	}
	// If not mapped, add the domain name to the list of mapped hosts for this IP
	d.mappedHosts[ipString] = append(d.mappedHosts[ipString], domainName)
}

func (d *DummyNetworkHost) createHostConfig() error {
	// First create a temporary host config file
	f, err := os.CreateTemp("", "dummy_hosts_*.conf")
	if err != nil {
		return fmt.Errorf("while creating temporary hosts config file: %w", err)
	}
	defer func() {
		err = f.Close()
	}()

	fmt.Fprintln(f, "# Dummy hosts config file for testing purposes") // Add a comment to the file for clarity
	// localhost
	fmt.Fprintln(f, "127.0.0.1 localhost")
	fmt.Fprintln(f, "127.0.0.1 iknite")
	fmt.Fprintln(f, "")
	for ip, domainNames := range d.mappedHosts {
		fmt.Fprintf(f, "%s %s\n", ip, strings.Join(domainNames, " "))
	}

	d.hostConfig = &txeh.HostsConfig{
		ReadFilePath:  f.Name(),
		WriteFilePath: f.Name(),
	}
	return nil
}

func (d *DummyNetworkHost) Cleanup() error {
	if d.hostConfig != nil {
		return os.Remove(d.hostConfig.ReadFilePath)
	}
	return nil
}

func NewDummyNetworkHost(ipAddresses []net.IP, mappedHosts map[string][]string) (*DummyNetworkHost, error) {
	r := &DummyNetworkHost{
		ipAddresses: ipAddresses,
		mappedHosts: mappedHosts,
	}
	err := r.createHostConfig()
	if err != nil {
		return nil, err
	}
	return r, nil
}

type DummySystem struct {
	Mounts []string
}

var _ System = (*DummySystem)(nil)

func (d *DummySystem) Unmount(path string) error {
	if !slices.Contains(d.Mounts, path) {
		return fmt.Errorf("path %s is not mounted", path)
	}
	// Simulate unmounting by removing the path from the list of mounts
	index := slices.Index(d.Mounts, path)
	d.Mounts[index] = d.Mounts[len(d.Mounts)-1] // Move the last element to the index of the removed element
	d.Mounts = d.Mounts[:len(d.Mounts)-1]       // Remove the last element
	return nil
}

func NewDummySystem(mounts []string) *DummySystem {
	return &DummySystem{
		Mounts: mounts,
	}
}

type DummyProcessState int

const (
	ProcessStateCreated DummyProcessState = iota
	ProcessStateRunning
	ProcessStateTerminated
	ProcessStateCompleted
)

func (d DummyProcessState) String() string {
	switch d {
	case ProcessStateCreated:
		return "Created"
	case ProcessStateRunning:
		return "Running"
	case ProcessStateTerminated:
		return "Terminated"
	case ProcessStateCompleted:
		return "Completed"
	default:
		return "Unknown"
	}
}

type DummyProcessOptions struct {
	Cmd      string
	Pid      int
	State    DummyProcessState
	ExitCode int
}

// DummyProcess simulates a process. It has a timeout of 1 minute, after which it will be considered as completed.
// It stops immediately whenever it receives the sigterm signal.
type DummyProcess struct {
	ctx        context.Context //nolint:containedctx // Used for global cancellation
	endChannel chan error
	cmd        string
	pid        int
	state      DummyProcessState
	exitCode   int
	mu         sync.Mutex
}

var (
	_ Process      = (*DummyProcess)(nil)
	_ ProcessState = (*DummyProcess)(nil)
)

// ExitCode implements [ProcessState].
func (p *DummyProcess) ExitCode() int {
	return p.exitCode
}

// Exited implements [ProcessState].
func (p *DummyProcess) Exited() bool {
	return p.state == ProcessStateCompleted || p.state == ProcessStateTerminated
}

// String implements [ProcessState].
func (p *DummyProcess) String() string {
	return fmt.Sprintf(
		"DummyProcess(pid=%d, cmd=%s, state=%s, exitCode=%d)",
		p.pid,
		p.cmd,
		p.state.String(),
		p.exitCode,
	)
}

// Success implements [ProcessState].
func (p *DummyProcess) Success() bool {
	return p.exitCode == 0
}

func (p *DummyProcess) Signal(signal os.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if signal == syscall.Signal(0) {
		if p.state == ProcessStateRunning {
			return nil
		}
		return fmt.Errorf("process is not running")
	}
	if signal == os.Interrupt || signal == os.Kill || signal == syscall.SIGTERM {
		p.state = ProcessStateTerminated
		p.endChannel <- nil // Signal that the process has ended
	}
	return nil
}

func (p *DummyProcess) Pid() int {
	return p.pid
}

func (p *DummyProcess) State() ProcessState {
	if p.state == ProcessStateCompleted || p.state == ProcessStateTerminated {
		return p
	}
	return nil
}

func (p *DummyProcess) Start(duration time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != ProcessStateCreated {
		return fmt.Errorf("process has already been started")
	}
	p.state = ProcessStateRunning
	// Start a goroutine to simulate the process running and completing after the specified duration
	go func() {
		time.Sleep(duration)
		if p.state == ProcessStateRunning {
			p.state = ProcessStateCompleted
			p.endChannel <- nil // Signal that the process has completed after the duration
		}
	}()
	return nil
}

func (p *DummyProcess) Wait() error {
	var unlockOnce sync.Once
	p.mu.Lock()
	defer unlockOnce.Do(p.mu.Unlock)
	if p.state == ProcessStateTerminated || p.state == ProcessStateCompleted {
		return nil
	}
	if p.state == ProcessStateCreated {
		return fmt.Errorf("process has not been started")
	}
	unlockOnce.Do(p.mu.Unlock) // Unlock before waiting to allow signal handling
	select {
	case err := <-p.endChannel:
		return err
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *DummyProcess) Cmd() string {
	return p.cmd
}

//nolint:contextcheck // The context is used for global cancellation of the process, not for individual method calls
func NewDummyProcess(ctx context.Context, options *DummyProcessOptions) *DummyProcess {
	if ctx == nil {
		ctx = context.Background()
	}
	// random pid
	return &DummyProcess{
		pid:        options.Pid,
		cmd:        options.Cmd,
		ctx:        ctx,
		state:      options.State,
		exitCode:   options.ExitCode,
		endChannel: make(chan error, 1),
	}
}

type DummyExecutor struct {
	Processes      map[int]Process
	fakeOutputs    map[string]string
	calledCommands []string
}

var _ Executor = (*DummyExecutor)(nil)

func (d *DummyExecutor) fakeOutput(stdin io.Reader, cmd string, args ...string) ([]byte, error) {
	key := cmd
	if len(args) > 0 {
		key += fmt.Sprintf(" %s", strings.Join(args, " "))
	}
	d.calledCommands = append(d.calledCommands, key)
	// Match the key as a regular expression to allow for flexible argument matching
	for pattern, output := range d.fakeOutputs {
		matched, err := regexp.MatchString(pattern, key)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		if matched {
			return []byte(output), nil
		}
	}
	output := &bytes.Buffer{}
	fmt.Fprintf(output, "result of executing %s with arguments %v on content:\n", cmd, args)
	if stdin != nil {
		_, err := io.Copy(output, stdin)
		if err != nil {
			return nil, fmt.Errorf("error while executing command: %w", err)
		}
	} else {
		fmt.Fprintln(output, "no input provided")
	}

	return output.Bytes(), nil
}

// ExecForEach implements [Executor].
func (d *DummyExecutor) ExecForEach(stdin *script.Pipe, cmd string) *script.Pipe {
	if stdin == nil {
		return script.NewPipe().WithError(fmt.Errorf("stdin is nil"))
	}
	tpl, err := template.New("").Parse(cmd)
	if err != nil {
		return stdin.WithError(err)
	}

	return stdin.Filter(func(r io.Reader, w io.Writer) error {
		scanner := newScanner(r)
		for scanner.Scan() {
			cmdLine := new(strings.Builder)
			err := tpl.Execute(cmdLine, scanner.Text())
			if err != nil {
				return err
			}
			output, err := d.fakeOutput(nil, cmdLine.String())
			if err != nil {
				return fmt.Errorf("while generating output of %s: %w", cmdLine, err)
			}
			fmt.Fprintln(w, string(output))
		}
		return scanner.Err()
	})
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), math.MaxInt)
	return scanner
}

// ExecPipe implements [Executor].
func (d *DummyExecutor) ExecPipe(stdin *script.Pipe, cmd string) *script.Pipe {
	if stdin == nil {
		stdin = script.NewPipe()
	}
	output, err := d.fakeOutput(stdin, cmd)
	if err != nil {
		return script.NewPipe().WithError(fmt.Errorf("error while generating fake output: %w", err))
	}
	return script.Echo(string(output))
}

// FindProcess implements [Executor].
func (d *DummyExecutor) FindProcess(pid int) (Process, error) {
	process, exists := d.Processes[pid]
	if !exists {
		return nil, fmt.Errorf("process with PID %d not found", pid)
	}
	return process, nil
}

// PipeRun implements [Executor].
func (d *DummyExecutor) PipeRun(stdin io.Reader, _ bool, cmd string, arguments ...string) ([]byte, error) {
	output, err := d.fakeOutput(stdin, cmd, arguments...)
	if err != nil {
		return nil, fmt.Errorf("error while generating fake output: %w", err)
	}
	return output, nil
}

// Run implements [Executor].
func (d *DummyExecutor) Run(_ bool, cmd string, arguments ...string) ([]byte, error) {
	output, err := d.fakeOutput(nil, cmd, arguments...)
	if err != nil {
		return nil, fmt.Errorf("error while generating fake output: %w", err)
	}
	return output, nil
}

// StartCommand implements [Executor].
func (d *DummyExecutor) StartCommand(ctx context.Context, options *CommandOptions) (Process, error) {
	process := NewDummyProcess(ctx, &DummyProcessOptions{
		Pid:      int(time.Now().UnixNano() % 10000),
		Cmd:      options.Cmd,
		State:    ProcessStateCreated,
		ExitCode: 0,
	})
	err := process.Start(10 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("error while starting command: %w", err)
	}
	d.Processes[process.Pid()] = process
	return process, nil
}

func (d *DummyExecutor) GetCalledCommands() []string {
	return d.calledCommands
}

func NewDummyExecutor(processes map[int]Process, fakeOutputs map[string]string) *DummyExecutor {
	return &DummyExecutor{
		Processes:   processes,
		fakeOutputs: fakeOutputs,
	}
}

type DummyHostOptions struct {
	FakeOutputs  map[string]string
	HostMappings map[string][]string
	Processes    []DummyProcessOptions
	Mounts       []string
	NetworkIPs   []net.IP
}

func NewDummyHost(
	fs FileSystem,
	options *DummyHostOptions,
) (*DelegateHost, error) {
	networkHost, err := NewDummyNetworkHost(options.NetworkIPs, options.HostMappings)
	if err != nil {
		return nil, fmt.Errorf("error while creating dummy network host: %w", err)
	}
	processes := make(map[int]Process)
	for _, processOptions := range options.Processes {
		process := NewDummyProcess(context.Background(), &processOptions)
		processes[process.Pid()] = process
	}
	r := &DelegateHost{
		Fs:   fs,
		Exec: NewDummyExecutor(processes, options.FakeOutputs),
		Sys:  NewDummySystem(slices.Clone(options.Mounts)),
		Net:  networkHost,
	}
	// Create the mount points in the dummy file system
	for _, mount := range options.Mounts {
		err = fs.MkdirAll(mount, os.FileMode(0o755))
		if err != nil {
			return nil, fmt.Errorf("error while creating mount point %s: %w", mount, err)
		}
	}
	// Create /proc/mounts file with the mounts for the dummy host
	f, err := fs.Create("/proc/self/mounts")
	if err != nil {
		return nil, fmt.Errorf("error while creating /proc/self/mounts file: %w", err)
	}
	defer func() {
		err = f.Close()
	}()
	for _, mount := range options.Mounts {
		fmt.Fprintf(f, "type %s type rw 0 0\n", mount)
	}
	err = fs.MkdirAll("/run/openrc/started", os.FileMode(0o755))
	if err != nil {
		return nil, fmt.Errorf("error while creating /run/openrc/started directory: %w", err)
	}
	// Create a service file for each process and a pid file
	for _, process := range options.Processes {
		pidFilePath := fmt.Sprintf("/run/%s.pid", process.Cmd)
		err = fs.WriteFile(pidFilePath, fmt.Appendf(nil, "%d", process.Pid), os.FileMode(0o644))
		if err != nil {
			return nil, fmt.Errorf("error while creating pid file for process %d: %w", process.Pid, err)
		}
		serviceFilePath := fmt.Sprintf("/run/openrc/started/%s", process.Cmd)
		err = fs.WriteFile(serviceFilePath, []byte{}, os.FileMode(0o644))
		if err != nil {
			return nil, fmt.Errorf("error while creating service file for process %d: %w", process.Pid, err)
		}
	}

	return r, nil
}
