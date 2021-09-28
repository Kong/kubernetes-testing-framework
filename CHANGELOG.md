# Changelog

## v0.7.0

### Improvements

* Added Istio as an available cluster addon.
  ([#122](https://github.com/Kong/kubernetes-testing-framework/pull/122))

### Breaking Changes

* Several public builder methods for the Kong cluster addon had name
  changes intended to make the naming more consistent and use prefixes
  as indices to improve readability and understanding of which components
  are being effected.
  ([#121](https://github.com/Kong/kubernetes-testing-framework/pull/121))

## v0.6.2

### Under The Hood

* Knative resources bumped to v0.18.0 for Kubernetes 1.22 compatibility.

## v0.6.1

### Improvements

* Utilities for generating and cleaning up transient testing namespaces
  were added in support of simplified setup in Golang test suites when
  using the KTF Go libraries for integration tests.
  ([#17](https://github.com/Kong/kubernetes-testing-framework/issues/17))

### Bug Fixes

* Fixed an issue where the Kong addon was not idempotent because adding
  the relevant helm repository could fail on re-entry despite the
  repository being present.
  ([#80](https://github.com/Kong/kubernetes-testing-framework/issues/80))

### Under The Hood

* Golang dependencies for several Kubernetes libraries were updated to
  the latest `v0.22.0` release (corresponds with Kubernetes `v1.22.0`
  release).
  ([k8s@v1.22.0](https://github.com/kubernetes/kubernetes/releases/tag/v1.22.0))

## v0.6.0

### Improvements

* Knative is now available as a cluster addon.
  ([#77](https://github.com/Kong/kubernetes-testing-framework/pull/77))

## v0.5.0

### Improvements

* GKE clusters created by KTF now get a label added that indicate that
  they were KTF-provisioned and by which IAM service account they were
  created by.
  ([#73](https://github.com/Kong/kubernetes-testing-framework/pull/73))

### Breaking Changes

* Removed a check when creating a cluster client that would validate
  that the /version endpoint of the cluster was up, as some use cases
  actually want to create the client first and then wait.
  ([#73](https://github.com/Kong/kubernetes-testing-framework/pull/73))

## v0.4.0

### Improvements

### Breaking Changes

* The `clusters.Cluster` interface now requires that implementations
  provide a method to retrieve the cluster version.
  [(#72](https://github.com/Kong/kubernetes-testing-framework/pull/72))

### Improvements

* Added utilities for auto-handling Ingress resources on older clusters.
  ([#70](https://github.com/Kong/kubernetes-testing-framework/pull/70))

## v0.3.3

### Improvements

* Deployed GKE clusters default to no addons enabled.
  ([#69](https://github.com/Kong/kubernetes-testing-framework/pull/69))

## v0.3.2

### Improvements

* Existing GKE clusters can now be loaded into a testing environment.
  ([#67](https://github.com/Kong/kubernetes-testing-framework/pull/67))

## v0.3.1

### Improvements

* The Kong addon now supports all service types where it previously
  only accepted (and assumed) type `LoadBalancer`.
  ([#64](https://github.com/Kong/kubernetes-testing-framework/pull/64))

## v0.3.0

### Improvements

* GKE Cluster implementation added.
  ([#32](https://github.com/Kong/kubernetes-testing-framework/issues/32))

### Breaking Changes

* Previously when KIND was the only Cluster implementation
  we defaulted to exposing the Kong Admin API via a LoadBalancer
  type service as this would not be accessible outside of the
  local docker network. Now that a GKE Cluster implementation
  exists this default would no longer be secure, so the default
  has been changed to ClusterIP.
  ([#32](https://github.com/Kong/kubernetes-testing-framework/issues/32))

## v0.2.2

### Security Fixes

* Containerd dependency updated to v1.4.8 to fix upstream security issue.
  ([GHSA-c72p-9xmj-rx3w](https://github.com/advisories/GHSA-c72p-9xmj-rx3w))
  ([#58](https://github.com/Kong/kubernetes-testing-framework/pull/60))

## v0.2.1

### Improvements

* Added Kubernetes version specification when building a
  new environments.Environment. This is now accessible in
  the CLI via `ktf env create --kubernetes-version <VER>`.
  ([#58](https://github.com/kong/kubernetes-testing-framework/pull/58))
