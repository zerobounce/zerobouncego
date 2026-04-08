# Contributing to Zero Bounce Go API

Thank you for your interest in contributing. This document explains how to get set up and submit changes.

## Code of Conduct

By participating in this project, you agree to uphold our [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting Started

See the [README](README.md) for prerequisites, setup, and how to run tests.

## How to Contribute

### Reporting Bugs

Open an [issue](https://github.com/zerobounce/zerobouncego/issues) and include:

* Environment details (Go version, OS)
* Steps to reproduce
* Expected vs actual behavior
* Relevant code or error messages

### Suggesting Changes

* Check existing issues and pull requests first.
* Open an issue to discuss larger changes or API design before coding.

### Submitting Changes

1. **Fork** the repository and create a branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and add or update tests where relevant.

3. **Run the test suite** (see README) before submitting.

4. **Commit** with a clear message (e.g. `Add X`, `Fix Y`).

5. **Push** your branch and open a **Pull Request** against `main`.

6. In the PR description, briefly explain what changed and why. Link any related issues.

Maintainers will review and may request changes. Once approved, your PR can be merged.

## Releases (maintainers)

Go modules are **not** published to a package registry. A release is a **git tag**; [proxy.golang.org](https://proxy.golang.org) mirrors tagged commits from GitHub.

**Critical rule:** the **`module` line in `go.mod` must match the major version of the tag**:

- **`v2.x.x` tags** require `module github.com/zerobounce/zerobouncego/v2`.
- **`v1.x.x` tags** use `module github.com/zerobounce/zerobouncego` (no `/v2`).

If you tag **v2+** without the **`/v2`** module path, the proxy will **not** serve that version (users see 404 / missing versions). See the full **Publish** section in [README.md](README.md) (checklist and `curl` verification) and [sdk-docs/pkg-go.dev](../sdk-docs/pkg-go-dev/) in the SDKs monorepo.

## Questions

* [Zero Bounce API docs](https://www.zerobounce.net/docs/)
* [Project homepage](https://zerobounce.net)
* Contact: **integrations@zerobounce.net**

Thanks for contributing.
