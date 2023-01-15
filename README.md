<!-- markdownlint-disable MD033 MD041 -->
<div id="top"></div>
<!-- markdownlint-enable MD033 MD041-->

<!-- PROJECT SHIELDS -->

[![Go Version][go-version]][go-version]
[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url] [![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![Apache 2.0 License][license-shield]][license-url]
[![Experimental][stability]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]
[![Workflow][workflow-shield]][workflow-url]

<!-- PROJECT LOGO -->
<!-- markdownlint-disable MD033 -->
<br />
<div align="center">

  <a href="https://github.com/kaweezle/iknite">
    <img src="images/logo.svg" alt="Logo" width="80" height="80">
  </a>
  <h3 align="center">Iknite</h3>

  <p align="center">
    Manages the initialization and start of a Kubernetes mono-machine cluster on Alpine Linux.
    <br />
    <a href="https://github.com/kaweezle/iknite"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://github.com/kaweezle/iknite/issues">Report Bug</a>
    ·
    <a href="https://github.com/kaweezle/iknite/issues">Request Feature</a>
  </p>
</div>

<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#built-with">How it works</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#installation">Installation</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#roadmap">Roadmap</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#contact">Contact</a></li>
    <li><a href="#acknowledgments">Acknowledgments</a></li>
  </ol>
</details>
<!-- markdownlint-enable MD033 -->
<!-- ABOUT THE PROJECT -->

## About The Project

Iknite manages the initialization and start of a Kubernetes cluster in an Alpine
based WSL 2 distribution or VM.

Iknite is part of the [kaweezle](https://github.com/kaweezle/kaweezle) project.
Kaweezle allows running a Kubernetes cluster on Windows using Windows Subsystem
for Linux 2 (WSL 2).

It is a small go based executable. It can be run from the command line or as an
openrc based service.

It is packaged as an APK that is published in its
[own repository](https://kaweezle.com/repo/).

It used by the [kaweezle-rootfs](https://github.com/kaweezle/kaweezle-rootfs)
project (now deprecated) and is included in the WSL root filesystem available
with each release (see below).

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

### How it works

Under the hood, iknite uses
[kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/)
to install the cluster, and [CRI-O](https://github.com/cri-o/cri-o) as the
container runtime. The APK package contains the relevant Alpine dependencies so
everything is already available when iknite is first launched.

With kubeadm, you get a vanilla Kubernetes installation, with its pros and cons.
There are more moving parts than in other light distributions but you won't be
blocked because some feature has been removed.

Some additional components are also installed:

- [flannel](https://github.com/flannel-io/flannel) as the CNI (Common Network
  Interface) provider
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server) for
  providing basic metrics to tools like K9s and for use by
  [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [MetalLB](https://metallb.universe.tf/) allowing to created a `LoadBalancer`
  service.
- [Local Path Provisioner](https://github.com/rancher/local-path-provisioner) to
  provide a default storage class bound to the local host filesystem.

This project uses the following popular [go](https://go.dev/) tools:

- [cobra](https://github.com/spf13/cobra)
- [logrus](github.com/sirupsen/logrus)
- [client-go](https://github.com/kubernetes/client-go)
- [goreleaser](https://goreleaser.com/) for building and packaging
- ... (take a look at `go.mod`)

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- GETTING STARTED -->

## Getting Started

Please refer to the
[kaweezle readme](https://github.com/kaweezle/kaweezle/README.md) for
installation instructions.

The following sections gives instructions on how to use the root filesystem
without the `kaweezle` windows command.

### Prerequisites

To run kaweezle, you'll need to have
[WSL installed](https://docs.microsoft.com/en-us/windows/wsl/install).

The simplest way to install it is to run the following command:

```powershell
> wsl --install
```

After reboot, update the kernel and set the default version to version 2:

```powershell
> sudo wsl --update
> wsl --set-default-version 2
```

For the other tools, you can use [Scoop](https://scoop.sh/).

To use the kubernetes cluster, you will need to have kubectl installed:

```powershell
> scoop install kubectl
```

**(Optional)** Other tools might be of interest, like `k9s`, `kubectx`, `kubens`
or `stern`. All are available through scoop. You can install all of them at once
with the following command:

```powershell
> scoop install k9s kubectx kubens stern
```

### Download and installation

The root filesystem can be downloaded from the
[Releases](https://github.com/kaweezle/iknite/releases) page.

You can create a WSL distribution with the following set of commands:

```powershell
PS> cd $env:LOCALAPPDATA
PS> mkdir kwsl
PS> cd kwsl
# 500Mb download ahead
PS> (New-Object Net.WebClient).DownloadFile("https://github.com/kaweezle/iknite/releases/latest/download/kaweezle.rootfs.tar.gz", "$PWD\rootfs.tar.gz")
PS> wsl --import kwsl . rootfs.tar.gz
...
PS>
```

You will have a WSL distribution called `kwsl` which file system will be located
in the current directory:

```powershell
❯ wsl -l -v
  NAME                    STATE           VERSION
* Alpine                  Stopped         2
  kwsl                    Stopped         2
```

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- USAGE EXAMPLES -->

## Usage

To start the kubernetes cluster, issue the following command:

```powershell
❯ wsl -d kwsl /sbin/iknite -v info start -w 120
INFO[0000] Adding IP address                             iface=eth0 ip=192.168.99.2/24
INFO[0000] Check domain name to IP mapping...            domainName=kaweezle.local ip=192.168.99.2
INFO[0001] Mapping not found, creating...                domainName=kaweezle.local ip=192.168.99.2
INFO[0001] Starting openrc...
INFO[0003] Running/usr/bin/kubeadm init --config /tmp/config419805402.yaml...
INFO[0012] Applying base kustomization...                directory=/etc/iknite.d
INFO[0014] Configuration applied                         directory=/etc/iknite.d resources="..."
INFO[0014] Waiting for workloads for 120 seconds...
INFO[0014] Workloads total: 7, ready: 0, unready:7       ready=0 total=7 unready=7
INFO[0016] Workloads total: 7, ready: 0, unready:7       ready=0 total=7 unready=7
...
INFO[0066] Workloads total: 7, ready: 6, unready:1       ready=6 total=7 unready=1
INFO[0068] Workloads total: 7, ready: 7, unready:0       ready=7 total=7 unready=0
INFO[0068] executed
```

The distribution is now running:

```powershell
❯ wsl -l -v
  NAME                    STATE           VERSION
* Alpine                  Stopped         2
  kwsl                    Running         2
```

Now the kubernetes cluster can be accessed:

<!-- cSpell:disable -->

```powershell
❯ $env:KUBECONFIG="\\wsl$\kwsl\root\.kube\config"
❯ kubectl get nodes
NAME             STATUS   ROLES    AGE     VERSION
kaweezle.local   Ready    <none>   3m22s   v1.26.0
❯ kubectl get pod -A
NAMESPACE            NAME                                     READY   STATUS    RESTARTS        AGE
kube-flannel         kube-flannel-ds-nf9z2                    1/1     Running   0               3m43s
kube-system          coredns-787d4945fb-vdv7s                 1/1     Running   0               3m43s
kube-system          coredns-787d4945fb-zxdnc                 1/1     Running   0               3m43s
kube-system          etcd-kaweezle.local                      1/1     Running   0               3m58s
kube-system          kube-apiserver-kaweezle.local            1/1     Running   0               3m58s
kube-system          kube-controller-manager-kaweezle.local   1/1     Running   0               3m57s
kube-system          kube-proxy-hgzvk                         1/1     Running   0               3m43s
kube-system          kube-scheduler-kaweezle.local            1/1     Running   0               3m56s
kube-system          metrics-server-699cfc467c-jn28p          1/1     Running   0               3m43s
local-path-storage   local-path-provisioner-8bc8875b-p7tqp    1/1     Running   0               3m43s
metallb-system       controller-577b5bdfcc-8grxd              1/1     Running   1 (3m10s ago)   3m43s
metallb-system       speaker-4p5b2                            1/1     Running   0               3m43s
```

<!-- cSpell:enable -->

You can now deploy anything on the cluster. As in contains
[MetalLB](https://metallb.universe.tf/), Any ingress controller (Traefik, for
instance) can be installed and be available locally.

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- ROADMAP -->

## Roadmap

See the [open issues](https://github.com/kaweezle/iknite/issues) for a full list
of proposed features (and known issues).

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- CONTRIBUTING -->

## Contributing

Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and
create a pull request. You can also simply open an issue with the tag
"enhancement". Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- LICENSE -->

## License

Distributed under the Apache License. See `LICENSE` for more information.

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- CONTACT -->

## Contact

Kaweezle - [@kaweezle](https://twitter.com/kaweezle)

Project Link:
[https://github.com/kaweezle/iknite](https://github.com/kaweezle/iknite)

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- ACKNOWLEDGMENTS -->

## Acknowledgements

This project started from the amazing work made by
[yuk7](https://github.com/yuk7) with [wsldl](https://github.com/yuk7/wsldl) and
[AlpineWSL](https://github.com/yuk7/AlpineWSL).

It also uses the great work made by the Alpine Linux community on the edge
repository.

You may be interested by existing alternatives from which we have taken some
ideas:

- [Rancher Desktop](https://rancherdesktop.io/)
- [Minikube](https://github.com/kubernetes/minikube)
- [Kind](https://kind.sigs.k8s.io/)

By using
[kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/)
and Alpine, kaweezle is closer to the clusters you may use on public clouds.

This readme has has been created from the
[Best-README-Template](https://github.com/othneildrew/Best-README-Template)
project.

<!-- markdownlint-disable-line --><p align="right">(<a href="#top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->

<!-- prettier-ignore-start -->
[contributors-shield]: https://img.shields.io/github/contributors/kaweezle/iknite.svg?style=for-the-badge
[contributors-url]: https://github.com/kaweezle/iknite/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/kaweezle/iknite.svg?style=for-the-badge
[forks-url]: https://github.com/kaweezle/iknite/network/members
[stars-shield]: https://img.shields.io/github/stars/kaweezle/iknite.svg?style=for-the-badge
[stars-url]: https://github.com/kaweezle/iknite/stargazers
[issues-shield]: https://img.shields.io/github/issues/kaweezle/iknite.svg?style=for-the-badge
[issues-url]: https://github.com/kaweezle/iknite/issues
[license-shield]: https://img.shields.io/badge/license-apache_2.0-green?style=for-the-badge&logo=none
[license-url]: https://github.com/kaweezle/iknite/blob/master/LICENSE
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/kaweezle
[go-version]: https://img.shields.io/badge/Go-1.17+-00ADD8?style=for-the-badge&logo=go
[stability]: https://img.shields.io/badge/stability-experimental-orange?style=for-the-badge
[workflow-shield]: https://github.com/kaweezle/iknite/actions/workflows/release.yml/badge.svg
[workflow-url]: https://github.com/kaweezle/iknite/actions/workflows/release.yml
<!-- prettier-ignore-end -->
