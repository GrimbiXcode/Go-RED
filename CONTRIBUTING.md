# Contributing to Go—RED

Thank you for your interest in contributing to Go—RED! We welcome contributions from everyone.

---

## Code of Conduct

By participating in this project, you agree to abide by the Code of Conduct.

---

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 18+
- Git

### Setting Up

1. Fork the repository on GitHub
2. Clone your fork locally:
```bash
git clone https://github.com/your-username/Go-RED.git
cd Go-RED
```
3. Set up the upstream remote:
```bash
git remote add upstream https://github.com/GrimbiXcode/Go-RED.git
```
4. Install all dependencies (checks your Go and Node versions first):
```bash
make setup
```
5. Build and run:
```bash
make start      # the finished app on http://localhost:8080
make dev        # or: both dev servers with hot reload, editor on http://localhost:5173
```
`make help` lists every command; `make check` runs everything CI runs.

---

## How to Contribute

- Report bugs with clear descriptions and reproduction steps
- Suggest features with use cases
- Submit pull requests with tests
- Improve documentation

---

## License

By contributing to Go—RED, you agree that your contributions will be licensed under the MIT License.
