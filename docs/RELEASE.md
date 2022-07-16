# Releases
The following documents the Flow release process.

# Binaries
Binaries are built for MacOS, Windows, and Linux for both `x86_64` and `arm64`. Flow uses [GoReleaser](https://goreleaser.com) to create the binaries:

First, ensure there is a new tag:

```
git tag -a -m '<tag>' <tag>
```

For example:

```
git tag -a -m 'v0.1.0' v0.1.0
```

Then use GoReleaser to build:

```
GITHUB_TOKEN="<TOKEN>" goreleaser release --rm-dist --skip-announce
```

# OCI Images
OCI images are built using the following command:

```
TAG=<TAG> UPDATE_LATEST=y PUSH=true IMAGE_BUILD_EXTRA="--builder=underland --platform linux/amd64,linux/arm64" VERSION=0.2.0 BUILD="" ./hack/build_images.sh
```
