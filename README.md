![ktf-banner](https://user-images.githubusercontent.com/5332524/120493758-39a54380-c389-11eb-8adb-ae4a30884851.png)

[![Unit Tests](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_unit.yaml/badge.svg)](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_unit.yaml)
[![Integration Tests](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_integration.yaml/badge.svg)](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_integration.yaml)
[![codecov](https://codecov.io/gh/Kong/kubernetes-testing-framework/branch/main/graph/badge.svg?token=ZJN2GM1CFS)](https://codecov.io/gh/Kong/kubernetes-testing-framework)
[![Go Report Card](https://goreportcard.com/badge/github.com/kong/kubernetes-testing-framework)](https://goreportcard.com/report/github.com/kong/kubernetes-testing-framework)
[![GoDoc](https://godoc.org/github.com/kong/kubernetes-testing-framework?status.svg)](https://godoc.org/github.com/kong/kubernetes-testing-framework)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/kong/kubernetes-testing-framework/blob/main/LICENSE)

# Kong Kubernetes Testing Framework (KTF)

Testing framework used by the [Kong Kubernetes Team][team] for the [Kong Kubernetes Ingress Controller (KIC)][kic].

[team]:https://github.com/orgs/Kong/teams/team-k8s
[kic]:https://github.com/kong/kubernetes-ingress-controller

# Requirements

* [Go][go] `v1.16.x+`

[go]:https://go.dev

# Usage

The following are some of the available features of the KTF:

- integration testing libraries for Kong on Kubernetes (Golang)
- unit testing libraries for the Kong Proxy (Golang)
- command line tool for testing environments and other testing features

For the integration testing libraries you have the option to deploy the Kong Proxy _only_ to the Kubernetes cluster or to deploy the entire stack depending on your testing needs.

## Command Line Tool

This project provides a command line tool `ktf` which can be used for reason such as building and maintaining a testing environment for Kong on Kubernetes.

### Install

If you have [Golang](https://go.dev) installed locally you can install with `go`:

```shell
$ go install github.com/kong/kubernetes-testing-framework/cmd/ktf@latest
```

Otherwise you can use the shell script to install the latest release for your operating system:

```shell
$ curl --proto '=https' -sSf https://kong.github.io/kubernetes-testing-framework/install.sh | sh
```

### Testing Environments

You can deploy a testing environment with the following command:

```shell
$ ktf environments create --generate-name
```

And it can be torn down with this command:

```shell
$ ktf environments delete --name <NAME>
```

#### Examples

The most common use cases will require some addon applications to be deployed to the cluster, particular actually deploying the [Kong Proxy](https://github.com/kong/kong) itself.

You can deploy a cluster with the Kong proxy already deployed and accessible via `LoadBalancer` services by running the following:

```shell
$ ktf environments create --name kong-proxy-testing --addon metallb --addon kong
```

Once the cluster is up configure your `kubectl` to use it:

```shell
$ kubectl cluster-info --context kind-kong-proxy-testing
```

You can see the IP addresses where you can reach the proxy and the Admin API with:

```shell
$ kubectl -n kong-system get services
```

# Contributing

See [CONTRIBUTING.md](/CONTRIBUTING.md).

# Community

If you have any questions about this tool and want to get in touch with the maintainers, check in on [#kong in Kubernetes Slack][slack].

[slack]:https://kubernetes.slack.com/messages/kong
