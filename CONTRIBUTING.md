# Contributing to Engram v3

Thank you for your interest in contributing to Engram v3! This document provides guidelines and instructions for contributing to this project.

## Development Environment Setup

1. **Prerequisites**
   - Go 1.21 or higher
   - Docker (for containerized development and testing)
   - Protocol Buffers compiler (protoc)
   - golangci-lint (for code linting)
   - pre-commit (for git hooks)

2. **Clone the Repository**
   ```bash
   git clone https://github.com/nkkko/engram-v3.git
   cd engram-v3
   ```

3. **Install Dependencies**
   ```bash
   go mod download
   ```

4. **Install Pre-commit Hooks**
   ```bash
   pre-commit install
   ```

## Project Structure

The project is organized into the following key directories:

- `cmd/` - Application entry points
- `internal/` - Internal packages not meant for external use
  - `api/` - HTTP/WebSocket API implementation
  - `config/` - Configuration loading and management
  - `domain/` - Core domain interfaces
  - `engine/` - Main engine coordinator
  - `lockmanager/` - Resource locking mechanism
  - `metrics/` - Prometheus metrics collection
  - `notifier/` - Real-time notification system
  - `router/` - Event routing system
  - `search/` - Text and metadata indexing
  - `storage/` - Persistent storage implementation
- `pkg/` - Public packages that can be imported by other projects
- `proto/` - Protocol Buffer definitions
- `tests/` - Integration and end-to-end tests

## Development Workflow

1. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Follow Go coding conventions
   - Ensure proper error handling
   - Add appropriate logging
   - Write tests for your changes

3. **Run Tests Locally**
   ```bash
   make test
   ```

4. **Run Linters**
   ```bash
   make lint
   ```

5. **Format Code**
   ```bash
   make fmt
   ```

6. **Commit Changes**
   - Use clear and descriptive commit messages
   - Reference issue numbers when applicable

7. **Submit a Pull Request**
   - Provide a clear description of your changes
   - Reference any related issues
   - Make sure all CI checks pass

## Coding Standards

1. **Code Style**
   - Follow standard Go style guidelines
   - Use `gofmt` and `goimports` to format code
   - Follow the project's existing patterns

2. **Error Handling**
   - Use idiomatic Go error handling
   - Add context to errors when propagating
   - Use structured logging with appropriate levels

3. **Testing**
   - Write unit tests for new code
   - Ensure integration tests cover major features
   - Benchmark performance-critical code

4. **Documentation**
   - Document all exported functions and types
   - Update README and other documentation when needed
   - Add examples for complex functionality

## License

By contributing to Engram v3, you agree that your contributions will be licensed under the same license as the project.

## Contact

If you have questions or need help, please open an issue on GitHub.

Thank you for contributing to Engram v3!