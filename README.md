# cossack_labs

### Generating code from proto files
docker run --volume "$(pwd):/workspace" --workdir /workspace bufbuild/buf generate --debug

### Linter
go tool revive ./...

### Test
go test -race ./...
