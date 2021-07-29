# Release Process

## Prerequisites

- [Git](https://git-scm.com/) `v2`

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

### TAG The Release

Given the tag determined previously, tag the `HEAD` of `main` (which should be the commit that was just made to update the changelog) with the determined release tag:

```shell
$ git tag vX.X.X
```

Where `vX.X.X` is the release tag previously determined.

Push `main` and the new tag to Github:

```shell
$ git push
$ git push --tags
```

### Enable The Release

A `release` workflow will be started when the tag is pushed, and will run unit, integration and e2e tests to validate the tagged release. You can watch the progress from the `Actions` tab:

  - https://github.com/Kong/kubernetes-testing-framework/actions/workflows/release.yaml

Once this completes, navigate to the release page:

  - https://github.com/Kong/kubernetes-testing-framework/releases

You will see a `draft` release that the release workflow made for the tag, which includes ta link to the changelog, and the release assets (binaries and checksums).

When you're ready to officially release the version, simply edit the draft and uncheck `pre-release` as needed and save the edit to publish it.
