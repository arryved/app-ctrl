# `app-control-api`

## Deploy
Create and push a semver-compatible tag to the repo; this will kick off the `goreleaser` workflow, which runs `goreleaser release --clean`, creates a release in this repo, and uploads the `.deb` artifact to the `arryved-apt` Google Artifact Repository. 

## Deploy (deprecated)
> [!NOTE]
>
> Leaving this here for posterity/until the new workflow is more formalized.

Ensure the version is updated both in `nfpm.yaml` and in the `Makefile`.

This will build then release the package to the `arryved-apt` repo:
```
make release
```
