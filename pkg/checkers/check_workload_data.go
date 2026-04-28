package checkers

import (
	"time"

	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

//nolint:interfacebloat // Interface is used to pass data between check and printer functions
type CheckWorkloadData interface {
	host.HostProvider
	IsOk() bool
	WorkloadCount() int
	ReadyWorkloads() []*v1alpha1.WorkloadState
	NotReadyWorkloads() []*v1alpha1.WorkloadState
	Iteration() int
	OkIterations() int
	Duration() time.Duration
	SetOk(bool)
	SetWorkloadCount(int)
	SetReadyWorkloads([]*v1alpha1.WorkloadState)
	SetNotReadyWorkloads([]*v1alpha1.WorkloadState)
	SetIteration(int)
	SetOkIterations(int)
	Start()
	ApiAdvertiseAddress() string
	ManifestDir() string
	WaitOptions() *utils.WaitOptions
}

type checkWorkloadData struct {
	startTime           time.Time
	waitOptions         *utils.WaitOptions
	alpineHost          host.Host
	apiAdvertiseAddress string
	readyWorkloads      []*v1alpha1.WorkloadState
	notReadyWorkloads   []*v1alpha1.WorkloadState
	workloadCount       int
	iteration           int
	okIterations        int
	ok                  bool
}

var _ CheckWorkloadData = (*checkWorkloadData)(nil)

func (c *checkWorkloadData) IsOk() bool {
	return c.ok
}

func (c *checkWorkloadData) WorkloadCount() int {
	return c.workloadCount
}

func (c *checkWorkloadData) ReadyWorkloads() []*v1alpha1.WorkloadState {
	return c.readyWorkloads
}

func (c *checkWorkloadData) NotReadyWorkloads() []*v1alpha1.WorkloadState {
	return c.notReadyWorkloads
}

func (c *checkWorkloadData) Iteration() int {
	return c.iteration
}

func (c *checkWorkloadData) OkIterations() int {
	return c.okIterations
}

func (c *checkWorkloadData) SetOk(ok bool) {
	c.ok = ok
}

func (c *checkWorkloadData) SetWorkloadCount(count int) {
	c.workloadCount = count
}

func (c *checkWorkloadData) SetReadyWorkloads(ready []*v1alpha1.WorkloadState) {
	c.readyWorkloads = ready
}

func (c *checkWorkloadData) SetNotReadyWorkloads(unready []*v1alpha1.WorkloadState) {
	c.notReadyWorkloads = unready
}

func (c *checkWorkloadData) SetIteration(iteration int) {
	c.iteration = iteration
}

func (c *checkWorkloadData) SetOkIterations(okIterations int) {
	c.okIterations = okIterations
}

func (c *checkWorkloadData) Start() {
	c.startTime = time.Now()
}

func (c *checkWorkloadData) Duration() time.Duration {
	if c.startTime.IsZero() {
		return 0
	}
	return time.Since(c.startTime)
}

func (c *checkWorkloadData) ApiAdvertiseAddress() string {
	return c.apiAdvertiseAddress
}

func (c *checkWorkloadData) ManifestDir() string {
	return kubeadmConstants.GetStaticPodDirectory()
}

func (c *checkWorkloadData) WaitOptions() *utils.WaitOptions {
	return c.waitOptions
}

func (c *checkWorkloadData) Host() host.Host {
	return c.alpineHost
}

func CreateCheckWorkloadData(
	apiAdvertiseAddress string,
	waitOptions *utils.WaitOptions,
	alpineHost host.Host,
) check.CheckData {
	return &checkWorkloadData{
		apiAdvertiseAddress: apiAdvertiseAddress,
		waitOptions:         waitOptions,
		alpineHost:          alpineHost,
	}
}
