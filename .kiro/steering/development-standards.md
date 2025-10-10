# Development Standards

## Language and Framework Preferences

- **Always first choice**: Write Go Lang using the AWS SDK v2 for Go
- Use good internal Go Lang subdirectory structure
- Include Makefile in project root

## Project Structure

- Keep as few files in the project root as possible
- When possible, work with existing files rather than creating new ones
- Put markdown summaries into a `./summaries` subdirectory

## Execution Modes

All applications must support two main modes of execution:

- Manual execution via CLI options
- Lambda-wrapper for event-driven workflows

## Logging

- Support structured logging with `slog`
- Feature both JSON and text output types

## Performance and Reliability

- All operations support full concurrency
- All operations support full API rate limiting with exponential backoff and retries
- All write/put/update/create/delete operations are always fully idempotent

## Operation Patterns

### Dry Run Support

- Read operations never implement `--dry-run`
- All write/put/update/create/delete operations always implement `--dry-run` to see what would happen without actually doing it

### Processing Patterns

- All operations can process one object or all objects
- When processing all objects, the code calls the process-one-object function multiple times
