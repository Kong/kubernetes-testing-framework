# Changelog

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
