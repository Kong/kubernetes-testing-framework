# Changelog

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
