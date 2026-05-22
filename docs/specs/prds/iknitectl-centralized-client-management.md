<!-- cSpell: words gitops oras paru -->

# Glossary

- **Workspace:** A workspace is related to a github repository. The repository
  contains:
  - A `values.yaml` file containing configuration values for the workspace.
  - A `secrets.sops.yaml` file containing the secrets of the workspace,
    encrypted with SOPS and stored in the root directory.
  - Files allowing the provisioning and deployment of components in the cluster.
- **Iknite cluster:** A kubernetes cluster powered by iknite. The cluster is
  identified by its name and the workspace it belongs to. It contains an ArgoCD
  instance, which is used to deploy the components of the workspace in the
  cluster.
- **Backend:** The backend is the technology used to provision the cluster.
  Examples of backends are Incus, Hyper-V, WSL, Docker, Openstack, etc.
- **Image:** The image is the root filesystem or the VM image used to provision
  the cluster. The image is stored in a docker registry and can be pulled with
  oras.
- **Git provider:** A git provider is the technology used to host the workspace
  repository. Examples of git providers are Github, Gitlab, Bitbucket, etc.
- **Gitops provider:** A gitops provider is the technology used to deploy the
  components of the workspace in the cluster. The gitops provider is installed
  in the cluster and configured to watch a specific branch or tag of the
  workspace repository. The only supported gitops provider is ArgoCD.

# Intent

We want to make iknitectl the _control center_ CLI for iknite powered kubernetes
clusters and workspaces. In particular, we want iknitectl to replace the
provisioning scripts:

- [Get-Iknite.ps1](../../../Get-Iknite.ps1)
- [get-iknite.sh](../../../get-iknite.sh)
- [Get-IkniteVM.ps1](../../../Get-IkniteVM.ps1)

Those scripts are currently used to pull the root filesystems or the VM images
and Incus metadata from the iknite registry and store them in the local
filesystem.

We want also iknitectl to manage the credentials related to the iknite clusters:

- The Certificate Authority(ies) (CA) certificate of the cluster(s). All the
  certificates of the cluster(s) are signed by one of these CA certificates.
- The ECDSA private key used with SOPS to encrypt and decrypt the secrets of the
  clusters and workspaces. The same key is used as the ssh key to connect to the
  clusters.

The credentials are stored in the iknitectl working directory, in the `auth`
subdirectory (to be discussed).

The environment also contains values and secrets that are shared between the
clusters and workspaces. For example, the openstack credentials are shared
between all the clusters and workspaces that use the openstack backend. These
values and secrets are stored in the `shared` subdirectory of the iknitectl
working directory.

**TBD**: It may be interesting to be able to retrieve shared information from a
centralized location. Examples of centralized locations are:

- A github repository. The information can be stored in a `shared` directory in
  the root of the repository. The information can be encrypted with SOPS and
  stored in a `shared/secrets.sops.yaml` file. The information can be decrypted
  with SOPS and stored in a `shared/values.yaml` file.
- A vault. The information can be stored in a vault path. The information can be
  encrypted

# Specification

## Subcommands

The following are subcommands, each related to a specific aspect of the cluster
management:

- `env`: Commands related to the management of the environment, such as the
  initialization of the iknitectl working directory and the management of the
  shared information.
- `image`: Commands related to the management of the images used to provision
  the clusters.
- `cluster`: Commands related to the management of the clusters.
- `workspace`: Commands related to the management of the workspaces.
- `auth`: Commands related to the management of the credentials of the clusters
  and workspaces.
- `backend`: Commands related to the management of the backends of the clusters.

The following are open to discussion.

- `sync`: Commands related to the synchronization of the clusters and workspaces
  with their corresponding github repositories.
- `deploy`: Commands related to the deployment of the components of the
  workspaces in the clusters.

Each subcommand has its own set of commands. For example, the `cluster`
subcommand has the following commands:

- `create`: Create a new cluster.
- `start`: Start a cluster.
- `stop`: Stop a cluster.
- `delete`: Delete a cluster.
- `sync-kubeconfig`: Synchronize the kubeconfig file of the cluster with the
  local kubeconfig file.

Each subcommand has a shortcut alias:

- `env` can be aliased as `e`.
- `image` can be aliased as `i` or `img`.
- `cluster` can be aliased as `c` or `cl`.
- `workspace` can be aliased as `w` or `ws`.
- `auth` can be aliased as `a`.
- `backend` can be aliased as `b` or `bck`.

**TBD**: There can be some shortcuts. For example, `iknitectl deploy` can be a
shortcut for `iknitectl workspace deploy`. Other possible shortcut is to to
imply `workspace` when the command is executed at the root of a workspace
repository. For example, if the user is in the root of a workspace repository
and executes `iknitectl deploy`, it is equivalent to
`iknitectl workspace deploy`.

## Data model

### Information related to a cluster

- The name of the cluster.
- The image used to provision the cluster. It can be a root filesystem or a VM
  image.
- The CA certificate of the cluster.
- The ECDSA private key used with SOPS to encrypt and decrypt the secrets of the
  cluster and workspace.
- The backend of the cluster. One of:
  - Incus
  - Incus VM
  - Hyper-V
  - WSL
  - Docker
  - Openstack
  - ... and more to come.
- The identifier of the cluster in the backend. For example, if the backend is
  Incus, the identifier is the name of the instance. If the backend is Hyper-V,
  the identifier is the name of the virtual machine.
- The workspace deployed on the cluster.
- The branch or tag of the workspace deployed on the cluster.

### Information related to an image

- The name of the image.
- The type of the image. One of:
  - Root filesystem
  - VM image
- The backend(s) compatible with the image. For example, a root filesystem image
  is compatible with the Incus and Docker backends, while a VM image is
  compatible with the Incus VM and Hyper-V backends.
- The URL of the image in the docker registry. The image can be pulled with
  oras.

### Information related to a backend

- The name of the backend.
- The type of the backend. One of:
  - Incus
  - Incus VM
  - Hyper-V
  - WSL
  - Docker
  - Openstack
  - ... and more to come.
- The configuration values of the backend. For example, the openstack backend
  requires the openstack credentials to be configured in the environment.

# Developer experience example

**Note**: The following examples imagine the developer is using an Arch Linux
distribution.

## Installation of iknitectl:

```bash
paru -S iknitectl
```

## Initialization of iknitectl:

```bash
iknitectl env init
```

The command creates the iknitectl working directory `$XDG_CONFIG_HOME/iknite`.
It also creates a default root CA certificate and a default ECDSA private key.

## Workspace creation

To create a new workspace:

```bash
iknitectl workspace create my-workspace --git-provider github --github-organization my-organization
cd my-workspace
```

or to initialize an existing repository:

```bash
iknitectl workspace init .
```

The command creates a new workspace in the current directory. It performs the
following steps:

- It creates a new github repository named `my-workspace` in the
  `my-organization` organization. The repository is initialized with a README
  file and a .gitignore file. The repository is also configured with a default
  branch named `main`.
- It clones the repository in the current directory and sets it as the workspace
  repository. The current branch is set to `main`. The command also configures
  the git user name and email for the repository.
- It creates a `values.yaml` file containing the default configuration values
  for the workspace and a `secrets.sops.yaml` file containing the default
  secrets for the workspace, encrypted with SOPS.

The command creates the following files in the current directory:

```bash
.
├── values.yaml                                     # contains the configuration values for the workspace
├── secrets.sops.yaml                               # contains the secrets for the workspace, encrypted with SOPS
├── .sops.yaml                                      # contains the SOPS configuration for the workspace
├── iknite-bootstrap.sh                             # contains the bootstrap script used by kubewait for the workspace
├── deploy/
    ├── ks8/
        ├── argocd/
            ├── common/
            ├── my-cluster/
                ├── appstages/
                    ├── appstage-00-bootstrap/     # contains the ArgoCD bootstrap apps of apps application
                       ├── application.yaml
                       ├── kustomization.yaml
```

## Cluster creation

```bash
iknitectl cluster create my-cluster --workspace . --backend incus --type container
```

This command creates a new cluster named `my-cluster` in the current workspace,
using the Incus backend and a root filesystem image. If not present, the command
also pulls the image from the iknite registry, store it in the local filesystem
and provision the cluster with it.

The container is configured to use the local CA certificate and the local ECDSA
private key. The cluster is also configured to bootstrap itself from the current
workspace repository on the current branch.

## Cluster status

```bash
iknitectl cluster status my-cluster
```

This command shows the status of the cluster, such as the backend status, the
ArgoCD status, the synchronization status of the cluster with the workspace
repository, etc.

## Cluster management

```bash
iknitectl cluster start my-cluster
iknitectl cluster stop my-cluster
iknitectl cluster delete my-cluster
```

## Kubeconfig cluster synchronization

```bash
iknitectl cluster sync-kubeconfig my-cluster
```

This command fetches the kubeconfig file of the cluster and merges it with the
local kubeconfig file. The command also sets the current context to the cluster.

## Cluster provisioning and deployment

```bash
iknitectl workspace deploy
```

# Implementation

- We want to use oras as a golang library to pull the docker images containing
  the root filesystems or the VM images and Incus metadata.

- The current commands of `iknitectl` would be moved to the workspace
  subcommand:
  - `iknitectl application ...` would become
    `iknitectl workspace application ...`.
  - `iknitectl install` would become `iknitectl workspace install`.
  - `iknitectl kustomize` would become `iknitectl workspace kustomize`.
  - `iknitectl secrets` would become `iknitectl workspace secrets`.

## Data storage

The credentials and shared information are stored in the iknitectl working
directory. On Linux, the working directory is `$XDG_CONFIG_HOME/iknite`. On
Windows, the working directory is `%APPDATA%\iknite`. On MacOS, the working
directory is `$HOME/Library/Application Support/iknite`.

```bash
$XDG_CONFIG_HOME/iknitectl/
├── auth/
└── shared/
```

The workspace information is stored in the github repository of the workspace.
The information is stored in the root directory of the repository:

```bash
my-workspace-repo/
├── values.yaml
├── secrets.sops.yaml
├── ...
```

**TBD**: Replace `values.yaml` with `iknite.yaml` ?

**TBD**: use a sqlite database to store the information related to the clusters
and images ? Use gorm as the ORM ? The database can be stored in the iknitectl
working directory.

## Workspace creation

The workspace is created from go templates embedded in the iknitectl binary. The
templates are rendered with the information provided by the user and the default
values. The rendered templates are then written to the workspace repository.

# Reference points

- Incus
