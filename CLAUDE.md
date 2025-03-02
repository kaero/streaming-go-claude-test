# CLAUDE.md - Project Guidelines

## Build & Run Commands
- Build: `go build -o streaming`
- Run server: `./streaming` or `go run main.go`
- Test: `go test ./...`
- Lint: `golangci-lint run`
- Format code: `go fmt ./...`

## Code Style Guidelines
- Follow standard Go conventions (gofmt)
- Use meaningful variable/function names in camelCase (or PascalCase for exports)
- Group imports: standard library first, then third-party
- Constants should be UPPER_SNAKE_CASE
- Errors: check immediately, return early, use descriptive messages
- Comments: all exported functions must have comments
- Concurrency: use mutexes for shared state, prefer channels for communication
- Limit line length to ~100 characters
- Use context for cancellation when appropriate
- Keep functions focused and under ~50 lines where possible

## Commit Guidelines
- Include the source prompts in all commit messages
- Format: Start with a brief summary, then add "Prompt: <original prompt>" on a new line