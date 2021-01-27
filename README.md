# cf-argo
codefresh argo cli

## Development
### Linting
we are using https://github.com/golangci/golangci-lint as our linter, you can integrate golangci-lint with the following IDEs:
* vscode: make sure `GOPATH` is setup correctly and run `make lint` this will download `golangci-lint` if it was not already installed on your machine. Then add the following to your `settings.json`:
```
"go.lintTool": "golangci-lint",
"go.lintFlags": [
    "--fix"
],
```