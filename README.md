![ktf-banner](https://user-images.githubusercontent.com/5332524/120493758-39a54380-c389-11eb-8adb-ae4a30884851.png)

[![tests](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/tests.yaml/badge.svg)](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/tests.yaml)
[![codecov](https://codecov.io/gh/Kong/kubernetes-testing-framework/branch/main/graph/badge.svg?token=ZJN2GM1CFS)](https://codecov.io/gh/Kong/kubernetes-testing-framework)
[![Go Report Card](https://goreportcard.com/badge/github.com/kong/kubernetes-testing-framework)](https://goreportcard.com/report/github.com/kong/kubernetes-testing-framework)
[![GoDoc](https://godoc.org/github.com/kong/kubernetes-testing-framework?status.svg)](https://godoc.org/github.com/kong/kubernetes-testing-framework)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/kong/kubernetes-testing-framework/blob/main/LICENSE)

# Kong Kubernetes Testing Framework (KTF)

Testing framework used by the [Kong Kubernetes Team][team].

Originally this testing framework was developed for the [Kong Kubernetes Ingress Controller (KIC)][kic] but is now used across multiple Kubernetes projects.

This testing framework supports the following use cases:

- **provide Kubernetes testing environments pre-deployed with addons for manual and automated testing via CLI or Golang**
- **provide unit testing utilities for Golang tests which focus on Kubernetes**
- **provide integration testing utilities for Golang tests which test Kubernetes controllers and other applications**

[team]:https://github.com/orgs/Kong/teams/team-k8s
[kic]:https://github.com/kong/kubernetes-ingress-controller

# Requirements

* [Go][go] `v1.18.x+`
* Linux (Mac/Windows not currently supported)

[go]:https://go.dev

# Usage

This framework can be used via command-line interface or as a Golang library.

## Command Line Tool

This project provides a command-line tool named `ktf` which can be used to build Kubernetes testing environments.

### Install

If you have [Golang](https://go.dev) installed locally you can install with `go`:

```shell
$ go install github.com/kong/kubernetes-testing-framework/cmd/ktf@latest
```

Otherwise you can use the shell script to install the latest release for your operating system:

```shell
$ curl --proto '=https' -sSf https://kong.github.io/kubernetes-testing-framework/install.sh | sh
```

If neither of these options suits you then you can install manually by navigating to the [RELEASES][releases] page and downloading the binary for your platform directly.

[releases]:https://github.com/Kong/kubernetes-testing-framework/releases

### Provisioning Kubernetes Testing Environments

You can deploy a testing environment with the following command:

```shell
$ ktf environments create --generate-name
```

Testing environments can be deleted with this command:

```shell
$ ktf environments delete --name <NAME>
```

#### Examples

Commonly this tool is used to deploy a Kubernetes enviroment with addons such as the [Kong Gateway](https://github.com/kong/kong).

You can deploy a cluster with the Kong Gateway already deployed and accessible via `LoadBalancer` services by running the following:

```shell
$ ktf environments create --name kong-gateway-testing --addon metallb --addon kong
```

Once the cluster is up configure your `kubectl` to use it:

```shell
$ kubectl cluster-info --context kind-kong-gateway-testing
```

You can see the IP addresses where you can reach the Gateway and the Admin API with:

```shell
$ kubectl -n kong-system get services
```

# Contributing

See [CONTRIBUTING.md](/CONTRIBUTING.md).

# Community

If you have any questions about this tool and want to get in touch with the maintainers, check in on [#kong in Kubernetes Slack][slack].

[slack]:https://kubernetes.slack.com/messages/kong
