# Releasing Forgelet

Use this checklist for a clean open-source release.

## 1) Pre-release Checks

- Update docs (`README.md`, command examples, changelog if used).
- Verify module path in `go.mod`:
  - `module github.com/lnyousif/forgelet`
- Ensure workspace is clean:

```bash
git status
```

- Run validation:

```bash
gofmt -w ./...
go test ./...
go build ./...
```

## 2) Cut Release

- Create and push a semver tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## 3) Publish GitHub Release

- Create a GitHub release from the tag.
- Add release notes:
  - Highlights
  - Breaking changes
  - Migration notes (if any)

## 4) Post-release Verify

- Verify install works:

```bash
go install github.com/lnyousif/forgelet/cmd/forgelet@v0.1.0
forgelet --help
```

- Announce release and include upgrade/install snippet.
