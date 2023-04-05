# Changelog

## v0.30.1

- Upgrade `metallb` addon to `v0.13.9`
  [#602](https://github.com/Kong/kubernetes-testing-framework/pull/602)

## v0.30.0

- Bump Kong Gateway Enterprise default image to 3.1.1.3
  [#566](https://github.com/Kong/kubernetes-testing-framework/pull/566)
- Added the ability to specify node machine type with GKE cluster builder.
  [#567](https://github.com/Kong/kubernetes-testing-framework/pull/567)

## v0.29.0

- Fix an endless loop in cleaner's `Cleanup()` when a namespace to be deleted
  is already gone.
  [#553](https://github.com/Kong/kubernetes-testing-framework/pull/553)
- Fix calico manifests URL
  [#555](https://github.com/Kong/kubernetes-testing-framework/pull/555)
- `clusters.Cleaner` can be used concurrently
  [#552](https://github.com/Kong/kubernetes-testing-framework/pull/552)

## v0.28.1

- Fix command error handling when error is `nil`
  [#527](https://github.com/Kong/kubernetes-testing-framework/pull/527)

## v0.28.0

- Add arm64 artifacts
  [#518](https://github.com/Kong/kubernetes-testing-framework/pull/518)
- Add a retry when deploying knative manifests and during kong chart installation
  [#520](https://github.com/Kong/kubernetes-testing-framework/pull/520)
  [#523](https://github.com/Kong/kubernetes-testing-framework/pull/523)

## v0.27.0

- fix cluster cleaner for Gateway API objects
  [#510](https://github.com/Kong/kubernetes-testing-framework/pull/510)

## v0.26.0

- gRPC API is used instead of gcloud CLI when createSubnetwork is enabled in
  GKE cluster builder.
  [#498](https://github.com/Kong/kubernetes-testing-framework/pull/498)
- GKE cluster builder accepts custom labels. 
  [#499](https://github.com/Kong/kubernetes-testing-framework/pull/499)

## v0.25.0

- GKE cluster builder allows creating a subnet for the cluster instead of using 
  a default one.
  [#490](https://github.com/Kong/kubernetes-testing-framework/pull/490)
- GKE cluster is able to wait for its cleanup synchronously. 
  [#491](https://github.com/Kong/kubernetes-testing-framework/pull/491)
- MetalLB addon will use an extended timeout when fetching manifests from GH which
  should improve its stability.
  [#492](https://github.com/Kong/kubernetes-testing-framework/pull/492)

## v0.24.1

- Golang dependencies for several Kubernetes libraries were updated to
  the latest `v0.26.0` release (corresponds with Kubernetes `v1.26.0`release).
  ([k8s@v1.26.0](https://github.com/kubernetes/kubernetes/releases/tag/v1.26.0))
- [sigs.k8s.io/yaml](https://github.com/kubernetes-sigs/yaml) is used as the only
  YAML library.
  [#463](https://github.com/Kong/kubernetes-testing-framework/pull/463)

## v0.24.0

- When available, calls to Github API, requesting the latest release from a repository
  will use `GITHUB_TOKEN` environment variable as token to authenticate against
  Github API.
  [#456](https://github.com/Kong/kubernetes-testing-framework/pull/456)
- Changed Kong addon to use udpProxy dict from charts value file to instantiate
  a udp proxy, instead of creating a udp proxy by kubectl.

## v0.23.0

- Upgrade `metallb` addon to `v0.13.6`

## v0.22.3

- Increased the metallb `IPAddressPool` IP range.

## v0.22.2

- Increased the metallb startup timeout.

## v0.22.1

### Fixed

- Moved metallb error recording inside context switch, to avoid
  recording the context expiration as the last error.

## v0.22.0

### Added

- loadimage addon now supports loading multiple images.
  [#391](https://github.com/Kong/kubernetes-testing-framework/pull/391)

### Fixed

- Delete `IPAddressPool` and `L2Advertisement` resources if the resource
  exists before creating in metallb addon.
  [#390](https://github.com/Kong/kubernetes-testing-framework/pull/390)


## v0.21.0

### Fixed

- Support k8s v1.25 in metallb addon by bumping to v0.13.5
  [#371](https://github.com/Kong/kubernetes-testing-framework/pull/371)

## v0.20.0

### Added

- Added the ability to select the `Service` type for the proxy when using the
  Kong addon via the Go library.
  [#346](https://github.com/Kong/kubernetes-testing-framework/pull/346)
- Added `WithProxyEnvVar()` Kong addon builder function, which sets environment
  variables for the proxy container.
  [#369](https://github.com/Kong/kubernetes-testing-framework/pull/369)

### Fixed

- Diagnostics now gets all resources, not the reduced set of resources returned
  by `kubectl get all`.
  [#362](https://github.com/Kong/kubernetes-testing-framework/pull/362)

## v0.19.0

### Added

- Added feature to support waiting for a port of a service to be connective
  by TCP.
  [#338](https://github.com/Kong/kubernetes-testing-framework/pull/338)

### Improved

- Increased retry times to increase the timeout to wait for kuma webhook
  to be ready to serve.
  [#341](https://github.com/Kong/kubernetes-testing-framework/pull/341)
  [#342](https://github.com/Kong/kubernetes-testing-framework/pull/342)

### Fixed

- Diagnostics runs the correct command for `kubectl describe`.
  [#343](https://github.com/Kong/kubernetes-testing-framework/pull/343)

## v0.18.0

### Added

- Added support for Postgres Kong config diagnostics and improved DB-less
  format.
  [#334](https://github.com/Kong/kubernetes-testing-framework/pull/334)
- The cleaner now has an `AddManifest()` function, to clean raw YAML manifests.
  [#334](https://github.com/Kong/kubernetes-testing-framework/pull/334)

## v0.17.0

### Breaking Changes

- `ApplyYAML` and `DeleteYAML` cluster utilities were renamed to
  `ApplyManifestByYAML` and `DeleteManifestByYAML`, respectively.
  [#330](https://github.com/Kong/kubernetes-testing-framework/pull/330)

### Added

- Added the ability to use Calico CNI instead of the default CNI for `kind`
  clusters. This can be triggered via the CLI using `--cni-calico` OR using the
  Go library with `WithCalicoCNI()`.
  [#330](https://github.com/Kong/kubernetes-testing-framework/pull/330)
- Added `ApplyManifestByURL` and `DeleteManifestByURL` helper functions to the
  `cluster` package as siblings to `ApplyYAML` and `DeleteYAML`.
  [#330](https://github.com/Kong/kubernetes-testing-framework/pull/330)
- Added a diagnostics system to collect resources, describe information, pod
  logs, and available plugin diagnostics.
  [#332](https://github.com/Kong/kubernetes-testing-framework/pull/332)

## v0.16.0

- Added `WithProxyImagePullSecret()` (`proxy-pull` with
  `KTF_TEST_KONG_PULL_USERNAME` and `KTF_TEST_KONG_PULL_PASSWORD` set on the
  CLI) feature to the Kong addon builder. It sets a pull secret for the proxy
  image.
  [#314](https://github.com/Kong/kubernetes-testing-framework/pull/314)

## v0.15.1

- Added missing Kuma addon CLI entry.
- Retry Kuma mesh installations to handle delayed webhook start.

### Improvements

- Update metallb to `v0.12.1`.

## v0.15.0

### Added

- The new Kuma addon installs the Kuma service mesh.
- `--kong-ingress-controller-image` selects the ingress controller image
  for the Kong addon.

### Fixed

- Disabled Google Ingress controller on GKE to avoid conflicts with other
  controllers.

## v0.14.2

### Added

- Added `clusters.KustomizeDeleteForCluster` utility to clean up kustomize
  deployed manifests that were deployed previously to a cluster.

## v0.14.1

### Added

- Added a `clusters.Cleaner` type which can be used to generically clean up
  cluster resources.

## v0.14.0

### Bug Fixes

- Kubernetes `v1.24.0` became the default recently for `kind` based clusters,
  which [had a backwards incompatible change][1.24.0-changelog] that caused new
  KTF builds to fail due to a [significant change in how ServiceAccounts worked
  which stopped their Secrets from being automatically generated][1.24.0-sas].
  A patch was issued to stop waiting for the default `ServiceAccount` to have a
  `Secret` to consider the cluster initialized.
  [#273](https://github.com/Kong/kubernetes-testing-framework/pull/273)

[1.24.0-changelog]:https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.24.md#no-really-you-must-read-this-before-you-upgrade
[1.24.0-sas]:https://github.com/kubernetes/enhancements/tree/master/keps/sig-auth/2799-reduction-of-secret-based-service-account-token

## v0.13.4

### Added

- The `--kong-gateway-image` flag can now be used with the CLI to signal which
  Kong Gateway container image to use in environments.

## v0.13.3

### Improvements

- various dependency updates

## v0.13.2

### Improvements

- Added `--kong-admin-service-loadbalancer` to the `ktf envs create`
  subcommand to make it easy to deploy the Kong Admin API as a
  `LoadBalancer` type `Service` when deploying with the Kong addon.
  [#245](https://github.com/Kong/kubernetes-testing-framework/pull/245)

## v0.13.1

### Bug Fixes

- Updates dependencies for relevant Docker and Containerd GHSA reports.
  [GHSA-qq97-vm5h-rrhg](https://github.com/advisories/GHSA-qq97-vm5h-rrhg)
  [GHSA-crp2-qrr5-8pq7](https://github.com/advisories/GHSA-crp2-qrr5-8pq7)

## v0.13.0

### Improvements

- Added `WithConfig()` to KIND cluster builder, which allows you to specify
  a custom KIND configuration. (see https://kind.sigs.k8s.io/docs/user/configuration/
  for the available configuration options).
  ([#222](https://github.com/Kong/kubernetes-testing-framework/pull/222))

## v0.12.1

### Bug Fixes

- Retry Knative install in the event that CRDs are not yet available.
  ([#209](https://github.com/Kong/kubernetes-testing-framework/pull/209))

## v0.12.0

### Improvements

- Knative defaults to the latest available version, and supports
  user-supplied versions.
  ([#196](https://github.com/Kong/kubernetes-testing-framework/pull/196))

## v0.11.1

### Improvements

- The default Kong Enterprise tag is now `2.7.0.0-alpine`.

## v0.11.0

### Bug Fixes

- Replicas exceeding the desired replica count (e.g. while a Deployment update
  spawns replacement replicas) no longer blocks Knative readiness.
  ([#177](https://github.com/Kong/kubernetes-testing-framework/pull/177))

### Improvements

- Namespace readiness checks confirm the presence of the namespace itself and
  the presence of Deployments within it.
  ([#166](https://github.com/Kong/kubernetes-testing-framework/pull/166))
- Addons can now indicate dependencies on other addons.
  ([#166](https://github.com/Kong/kubernetes-testing-framework/pull/166))
- Kong addon instances now listen on TCP port 8899 for TLS connections.
  ([#167](https://github.com/Kong/kubernetes-testing-framework/pull/167))
- Added registry addon to provide a local Docker registry within the test
  cluster.
  ([#170](https://github.com/Kong/kubernetes-testing-framework/pull/170))

## v0.10.0

### Bug Fixes

- Fixed a readiness timing issue with cert-manager wherein the webhook
  could be unready when the addon reports as ready.
  ([#159](https://github.com/Kong/kubernetes-testing-framework/issues/159))

### Improvements

- Added a [CertManager](https://cert-manager.io/) addon.
  ([#148](https://github.com/Kong/kubernetes-testing-framework/pull/148))
- Added a utility function to invoke `kubectl wait --for-condition=CONDITION`.
  ([#148](https://github.com/Kong/kubernetes-testing-framework/pull/148))
- Added a utility function to delete a YAML manifest from the cluster.
  ([#148](https://github.com/Kong/kubernetes-testing-framework/pull/148))
- Added an addon to load images into the test cluster from a local Docker
  environment.
  ([#151](https://github.com/Kong/kubernetes-testing-framework/pull/151))

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
