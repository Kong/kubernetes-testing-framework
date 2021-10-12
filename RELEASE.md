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

### Using a tag to trigger a release

Given the tag determined previously, tag the `HEAD` of `main` (which should be the commit that was just made to update the changelog) with the determined release tag:

```shell
$ git tag do-release-vX.X.X
```

Where `vX.X.X` is the release tag previously determined.

Push `main` and the new tag to Github:

```shell
$ git push
$ git push --tags
```

This will result in a `release-testing` workflow which upon success will push a `vX.X.X` tag for the release which will trigger the `release` workflow.

The `release` workflow will create a Github release and upload artifacts to that release, but will leave it in a `draft state` and it will need to be enabled manually.

#### Resolving Workflow Errors

Sometimes the `release-testing` workflow may fail for one reason or another, which is why we trigger releases with a `do-release-*` tag.

If something does go wrong however and you need to trigger a new release for the same `vX.X.X` version, you can push any number of additional tags with the following format:

- `do-release-attempt-<NUMBER>-vX.X.X`

The `<NUMBER>` part is purely arbitrary but by convention you can just make this an incrementing counter until the `release-testing` workflow completes successfully.

Note that all `do-release-*` tags that are pushed will be automatically cleaned up by CI, you can forget about them once they're pushed.

### Enable The Release

Navigate to the release page:

  - https://github.com/Kong/kubernetes-testing-framework/releases

You will see a `draft` release that the release workflow made for the tag, which includes ta link to the changelog, and the release assets (binaries and checksums).

When you're ready to officially release the version, simply edit the draft and uncheck `pre-release` as needed and save the edit to publish it.

## Notes

- Under the hood the `release-testing` workflow includes a script to re-tag and push the release from `do-release-v*` tags. If you would like to see the documentation for this script run `go doc internal/ci/release/tagging/main.go`
