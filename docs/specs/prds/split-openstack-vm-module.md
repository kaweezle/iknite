## Intent

Currently, the OpenStack VM module OpenStack VM module
[deploy/iac/modules/openstack-vm](../../../deploy/iac/modules/openstack-vm/)
handles multiple responsibilities:

- It creates the iknite test image on OpenStack.
- It provisions a test VM instance using the created image and waits for it to
  become reachable via SSH.

We want to add an end-to-end test that involves:

- Waiting for the kubernetes cluster to become ready after bootstrapping
- Deploy ArgoCD onto the cluster via its official Helm chart.
- Make ArgoCD **auto-pilot** itself by pointing it to the iknite Git repository
  and the appropriate path within that repository.
- Make ArgoCD deploy an ingress controller and the resources that expose the
  ArgoCD dashboard.
- Test that the ArgoCD dashboard is reachable.

In order to achieve this, we need to split the existing OpenStack VM module into
smaller modules with single responsibilities. This will allow us to reuse the VM
creation logic in the new end-to-end test module without duplicating code.

The purpose of this document is to outline the plan for splitting the existing
OpenStack VM module
[deploy/iac/modules/openstack-vm](../../../deploy/iac/modules/openstack-vm/)
into smaller, more manageable modules. This will enhance maintainability,
reusability, and clarity of the codebase.

## Specification

The existing OpenStack VM module will be split into the following smaller
modules:

1. **OpenStack Image Module**: This module will be responsible for creating the
   iknite test image on OpenStack. It will handle all tasks related to image
   creation, including uploading the image and configuring its properties. It
   will be named `openstack-image` and located at
   [deploy/iac/modules/openstack-image](../../../deploy/iac/modules/openstack-image/)
2. **OpenStack VM Module**: This module will be responsible for provisioning a
   test VM instance using a specified image. It will handle tasks such as
   instance creation, network configuration, and SSH access setup. It will be
   named `openstack-vm` and located at
   [deploy/iac/modules/openstack-vm](../../../deploy/iac/modules/openstack-vm/)

`openstack-image` will output the ID of the created image, which will be used as
an input to the `openstack-vm` module. This will allow the `openstack-vm` module
to provision a VM instance using the image created by the `openstack-image`
module.

The existing terragrunt unit
[`deploy/iac/iknite/iknite-vm`](../../../deploy/iac/iknite/iknite-vm/) needs
also to be split into two separate terragrunt units:

1. **OpenStack Image Terragrunt Unit**: This unit will utilize the
   `openstack-image` module to create the iknite test image on OpenStack. It
   will be named `iknite-image` and located at
   [deploy/iac/iknite/iknite-image](../../../deploy/iac/iknite/iknite-image/)
2. **OpenStack VM Terragrunt Unit**: This unit will utilize the `openstack-vm`
   module to provision a test VM instance using the image created by the
   `iknite-image` unit. It will be named `iknite-vm` and located at
   [deploy/iac/iknite/iknite-vm](../../../deploy/iac/iknite/iknite-vm/)

## Implementation

- Implement the `openstack-image` module to handle image creation on OpenStack.
- Implement the `openstack-vm` module to handle VM provisioning using a
  specified image.
- Create the `iknite-image` terragrunt unit to utilize the `openstack-image`
  module.
- Modify the existing `iknite-vm` terragrunt unit to utilize the new
  `openstack-vm` module and accept the image ID from the `iknite-image` unit as
  input (create the dependency).

## Tests

- Fist test the `iknite-image` unit independently to ensure it correctly creates
  the iknite test image on OpenStack.
- Then test the `iknite-vm` unit independently to ensure it correctly provisions

## Pre-commit Checks

Run the following pre-commit checks to ensure code quality and consistency:

- `pre-commit run --all-files`
