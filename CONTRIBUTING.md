# Contributing

Thanks for contributing to Forgelet.

## Development Setup

```bash
cd forgelet
go mod download
go build ./cmd/forgelet
```

## Code Style

- Use idiomatic Go naming and package layout.
- Run formatting before opening a PR:

```bash
gofmt -w ./...
```

- Keep changes focused and minimal.

## Before Opening a PR

- Ensure code builds locally:

```bash
go build ./...
```

- Smoke test CLI help:

```bash
go run ./cmd/forgelet --help
```

- Update docs when behavior/commands change.

## Pull Request Checklist

- Clear title and summary
- Linked issue (if applicable)
- Notes on breaking changes (if any)
- Updated docs/examples
