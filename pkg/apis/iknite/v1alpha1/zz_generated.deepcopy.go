//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright Antoine Martin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	net "net"

	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterWorkloadsState) DeepCopyInto(out *ClusterWorkloadsState) {
	*out = *in
	if in.Ready != nil {
		in, out := &in.Ready, &out.Ready
		*out = make([]*WorkloadState, len(*in))
		copy(*out, *in)
	}
	if in.Unready != nil {
		in, out := &in.Unready, &out.Unready
		*out = make([]*WorkloadState, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterWorkloadsState.
func (in *ClusterWorkloadsState) DeepCopy() *ClusterWorkloadsState {
	if in == nil {
		return nil
	}
	out := new(ClusterWorkloadsState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IkniteCluster) DeepCopyInto(out *IkniteCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IkniteCluster.
func (in *IkniteCluster) DeepCopy() *IkniteCluster {
	if in == nil {
		return nil
	}
	out := new(IkniteCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IkniteCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IkniteClusterSpec) DeepCopyInto(out *IkniteClusterSpec) {
	*out = *in
	if in.Ip != nil {
		in, out := &in.Ip, &out.Ip
		*out = make(net.IP, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IkniteClusterSpec.
func (in *IkniteClusterSpec) DeepCopy() *IkniteClusterSpec {
	if in == nil {
		return nil
	}
	out := new(IkniteClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IkniteClusterStatus) DeepCopyInto(out *IkniteClusterStatus) {
	*out = *in
	in.WorkloadsState.DeepCopyInto(&out.WorkloadsState)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IkniteClusterStatus.
func (in *IkniteClusterStatus) DeepCopy() *IkniteClusterStatus {
	if in == nil {
		return nil
	}
	out := new(IkniteClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadState) DeepCopyInto(out *WorkloadState) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadState.
func (in *WorkloadState) DeepCopy() *WorkloadState {
	if in == nil {
		return nil
	}
	out := new(WorkloadState)
	in.DeepCopyInto(out)
	return out
}
