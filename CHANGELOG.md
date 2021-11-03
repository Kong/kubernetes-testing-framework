# Changelog

## v0.9.1

### Improvements

- Enterprise license utilities were added for the Kong cluster addon.
  ([#144](https://github.com/Kong/kubernetes-testing-framework/pull/144))

## v0.9.0

### Improvements

- Cluster utilities were added to apply raw YAML or Kustomize configurations
  to a cluster object for convenience.
  ([#135](https://github.com/Kong/kubernetes-testing-framework/pull/135))

### Breaking Changes

- Several helper functions in the kubernetes `generators` package which were
  centered around cluster related functionality have been moved to the
  `clusters` package (e.g. `TempKubeconfig()`, `GenerateNamespace()`,
  `CleanupGeneratedResources()`, and `TestGenerators()`)
  ([#135](https://github.com/Kong/kubernetes-testing-framework/pull/135))

## v0.8.3

### Bug Fixes

- Fixed the CLI `main.go` location to fix `go install`
  ([#133](https://github.com/Kong/kubernetes-testing-framework/pull/133))

### Under The Hood

- Added release tooling for pipelining releases in Github Actions CI
  ([#134](https://github.com/Kong/kubernetes-testing-framework/pull/134))

## v0.8.2

### Under The Hood

- CI improvements for releasing pipelining were the only changes made, so this
  release is simply an exercise of those changes.

## v0.8.1

### Bug Fixes

- The Istio addon now retries deployment of components such as Kiali to deal
  with order of operations issues found in some older Istio releases. This
  fixes compatibility with `v1.9` and `v1.10`.
  ([#130](https://github.com/Kong/kubernetes-testing-framework/pull/130))

### Under The Hood

- containerd Go dependency updated to `v1.4.11`
- docker Go dependency updated to `v20.10.9`

## v0.8.0

### Improvements

* [HttpBin][httpbin] is now an available addon (also available via the CLI).
  ([#127](https://github.com/Kong/kubernetes-testing-framework/pull/127))
* The [Istio][istio] addon is now available via the CLI.
  ([#127](https://github.com/Kong/kubernetes-testing-framework/pull/127))
* Networking testing utils now include HTTP testing functions.
  ([#127](https://github.com/Kong/kubernetes-testing-framework/pull/127))

[httpbin]:https://github.com/postmanlabs/httpbin
[istio]:https://istio.io

### Under The Hood

* General stability improvements to Addon readiness functionality.
  ([#127](https://github.com/Kong/kubernetes-testing-framework/pull/127))

## v0.7.2

### Bug Fixes

* Fixed a bug with generation of secrets for enterprise enabled Kong addons
  which would occasionally cause the addon to fail to deploy.
  ([#125](https://github.com/Kong/kubernetes-testing-framework/pull/125))

### Under The Hood

* Integration test parallelization was re-tuned according to some problems
  that were found with running multiple kind clusters in Github Actions.
  ([#125](https://github.com/Kong/kubernetes-testing-framework/pull/125))

## v0.7.1

### Under The Hood

* This release was entirely CI related and has no end-user effect.

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
