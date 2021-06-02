![ktf-banner](https://user-images.githubusercontent.com/5332524/120493758-39a54380-c389-11eb-8adb-ae4a30884851.png)

[![Unit Tests](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_unit.yaml/badge.svg)](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_unit.yaml)
[![Integration Tests](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_integration.yaml/badge.svg)](https://github.com/Kong/kubernetes-testing-framework/actions/workflows/test_integration.yaml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/kong/kubernetes-testing-framework/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/kong/kubernetes-testing-framework)](https://goreportcard.com/report/github.com/kong/kubernetes-testing-framework)
[![GoDoc](https://godoc.org/github.com/kong/kubernetes-testing-framework?status.svg)](https://godoc.org/github.com/kong/kubernetes-testing-framework)

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

For the integration testing libraries you have the option to deploy the Kong Proxy _only_ to the Kubernetes cluster or to deploy the entire stack depending on your testing needs.

# Contributing

See [CONTRIBUTING.md](/CONTRIBUTING.md).

# Community

If you have any questions about this tool and want to get in touch with the maintainers, check in on [#kong in Kubernetes Slack][slack].

[slack]:https://kubernetes.slack.com/messages/kong
