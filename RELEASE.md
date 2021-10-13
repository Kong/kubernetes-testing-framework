# Release Process

## Prerequisites

- [Git](https://git-scm.com/) `v2`
- A `GITHUB_TOKEN` with `repo:write` permissions

## Github Release

The vast majority of the release process is automated via the `release.yaml` [Github Actions](https://github.com/features/actions) workflow, a maintainer only needs to perform a couple steps.

### Determine Release TAG

Release tags are based on [Semantic Versioning (semver)](https://semver.org/).

The `CHANGELOG.md` file should include any breaking or backwards incompatible changes since the last release.

If there are any breaking changes the upcoming release must increment the `MAJOR` version.

**CAVEAT**: during the `v0.x.x` release cycle breaking changes need only update the `MINOR` release version, after `v1` this changes.

### Update the CHANGELOG.md

Edit the `CHANGELOG.md` file and add the release or modify the release as needed. Ensure the new `TAG` is represented there and use previous tags as a guide.

Make a commit directly to `main` with a message following this pattern:

```shell
$ git commit -m 'docs: update CHANGELOG.md for release vX.X.X' CHANGELOG.md
```

Where `vX.X.X` is the release tag you'll be making.

### Release Testing & Tagging

Given the `vX.X.X` tag from above (which should not yet have been made) you can trigger release testing and creation of the tag with the following:

```console
$ GITHUB_TOKEN=<YOUR_TOKEN> GITHUB_ORG=kong GITHUB_REPO=kubernetes-testing-framework go run internal/ci/workflows/main.go release-testing.yaml main tag=vX.X.X
```

The above will start a workflow that runs the release tests, when they succeed the workflow will create the `vX.X.X` tag and then you can complete the release.

**NOTE**: sometimes the release tests fail for one reason or another and you may need to make some additional patches, in that case the above command can be run multiple times until the release testing succeeds and the tag is pushed.

**NOTE**: if you want to see the documentation for the above script run `go doc internal/ci/workflows/main.go`

### Github Release

Once the above release testing has succeeded and pushed your tag, you may trigger the final release:

```console
$ GITHUB_TOKEN=<YOUR_TOKEN> GITHUB_ORG=kong GITHUB_REPO=kubernetes-testing-framework go run internal/ci/workflows/main.go release.yaml vX.X.X
```

The resulting workflow will create the final Github Release and will build and publish the release artifacts for it, and the release is complete.
