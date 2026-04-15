package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/utils"
)

func TestDifference(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	diff := difference([]string{"a", "b", "c"}, []string{"b", "x"})
	req.Equal([]string{"a", "c"}, diff)
}

func TestFileTreeDifferenceAndCheck(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := t.TempDir()
	req.NoError(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600))
	req.NoError(os.MkdirAll(filepath.Join(dir, "sub"), 0o700))
	req.NoError(os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0o600))

	expected := []string{"a.txt", filepath.Join("sub", "b.txt")}
	missing, extra, err := FileTreeDifference(dir, expected)
	req.NoError(err)
	req.Empty(missing)
	req.Empty(extra)

	check := FileTreeCheck("tree", "tree check", dir, expected)
	ok, msg, err := check.CheckFn(context.Background(), nil)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "All expected")

	badCheck := FileTreeCheck("tree", "tree check", dir, []string{"missing"})
	ok, msg, err = badCheck.CheckFn(context.Background(), nil)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "Missing files")
}

func TestKubernetesFileCheckAndSystemFileCheck(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "conf.txt")
	req.NoError(os.WriteFile(file, []byte("ok\n"), 0o600))

	kubeCheck := KubernetesFileCheck("kube-file", file)
	ok, msg, err := kubeCheck.CheckFn(context.Background(), nil)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "exists and is a file")

	contentCheck := SystemFileCheck("sys-file", "desc", file, "ok")
	ok, msg, err = contentCheck.CheckFn(context.Background(), nil)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "expected content")

	badeCheck := SystemFileCheck("sys-file", "desc", file, "bad")
	ok, _, err = badeCheck.CheckFn(context.Background(), nil)
	req.Error(err)
	req.False(ok)
}

func TestCheckWorkloadDataAccessors(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	waitOptions := utils.NewWaitOptions()
	raw := CreateCheckWorkloadData("10.0.0.1", waitOptions, alpine.NewDefaultAlpineHost())
	data, ok := raw.(*checkWorkloadData)
	req.True(ok)

	ready := []*v1alpha1.WorkloadState{{Namespace: "ns", Name: "r", Ok: true, Message: "ok"}}
	unready := []*v1alpha1.WorkloadState{{Namespace: "ns", Name: "u", Ok: false, Message: "pending"}}

	data.SetOk(true)
	data.SetWorkloadCount(2)
	data.SetReadyWorkloads(ready)
	data.SetNotReadyWorkloads(unready)
	data.SetIteration(3)
	data.SetOkIterations(2)
	req.Zero(data.Duration())
	data.Start()
	time.Sleep(2 * time.Millisecond)

	req.True(data.IsOk())
	req.Equal(2, data.WorkloadCount())
	req.Equal(ready, data.ReadyWorkloads())
	req.Equal(unready, data.NotReadyWorkloads())
	req.Equal(3, data.Iteration())
	req.Equal(2, data.OkIterations())
	req.Equal("10.0.0.1", data.ApiAdvertiseAddress())
	req.NotNil(data.WaitOptions())
	req.Greater(data.Duration(), time.Duration(0))
}

func TestWorkloadResultPrinters(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	state := &v1alpha1.WorkloadState{Namespace: "kube-system", Name: "coredns", Message: "Ready", Ok: true}
	line := PrettyPrintWorkloadState("  ", state)
	req.Contains(line, "kube-system")
	req.Contains(line, "coredns")

	data := &checkWorkloadData{waitOptions: utils.NewWaitOptions()}
	data.SetWorkloadCount(2)
	data.SetReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "a", Ok: true, Message: "ok"}})
	data.SetNotReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "b", Ok: false, Message: "pending"}})
	data.Start()

	result := &CheckResult{
		Check:     &Check{Name: "workloads", Description: "workloads"},
		Status:    StatusRunning,
		CheckData: data,
	}

	out := CheckWorkloadResultPrinter(result, "", "*")
	req.Contains(out, "workloads")
	req.Contains(out, "ns")

	result.Status = StatusSkipped
	out = CheckWorkloadResultPrinter(result, "", "*")
	req.Contains(out, "workloads")

	fallback := &CheckResult{Check: &Check{Name: "x", Description: "x"}, Status: StatusSuccess, CheckData: "bad-data"}
	req.Contains(CheckWorkloadResultPrinter(fallback, "", "*"), "x")
}

func TestAdditionalCheckerPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// TODO: Use mocks and make a full test of CheckService.
	h := alpine.NewDefaultAlpineHost()

	ok, msg, err := CheckService(h, "ignored", false, false)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "Service ignored is running")

	ok, _, err = CheckApiServerHealth(time.Millisecond, "invalid")
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckWorkloads(context.Background(), "invalid")
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckIkniteServerHealth(context.Background(), &utils.WaitOptions{Wait: false, Watch: false})
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckKubeletHealth(time.Millisecond)
	req.Error(err)
	req.False(ok)

	serviceCheck := ServiceCheck("svc", "containerd")
	req.NotNil(serviceCheck)
	req.Equal("svc", serviceCheck.Name)
}
