

## Continuous Integration

I use GitHub Actions (defined in `.github/workflows/ci.yml`) to run all Go unit tests on every push and pull request to `main`.  
The workflow checks out the code, sets up Go, downloads dependencies, and executes:

```bash
go test -v ./...
```