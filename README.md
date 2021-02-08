# cf-argo
codefresh argo cli

## Development

### Building from Source
To build a binary from the source code, make sure you have `go >=1.15` installed and that the `GOPATH` environment variable is set, the run: `make` to build the binary to `./dist/` or `make install` to make it available as `cf-argo` in the `PATH`.
### Linting
We are using https://github.com/golangci/golangci-lint as our linter, you can integrate golangci-lint with the following IDEs:
* vscode: make sure `GOPATH` is setup correctly and run `make lint` this will download `golangci-lint` if it was not already installed on your machine. Then add the following to your `settings.json`:
```
"go.lintTool": "golangci-lint",
"go.lintFlags": [
    "--fast"
],
```
