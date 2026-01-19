# Contributing to Iknite

<!-- cspell:words venv mvenv Riseup Susam -->

Thank you for considering contributing to Iknite! We welcome contributions from
the community to help improve the project.

## Code of Conduct

This project and everyone participating in it is governed by the
[Iknite Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected
to uphold this code. Please report unacceptable behavior to
[antoine@mrtn.me](mailto:antoine@mrtn.me).

## Getting Started

### Branch Naming Conventions

- `main` - Main development branch
- `deploy/*` - Continuous deployment branches (created by CI/CD)
- Use lowercase with hyphens (`-`) instead of underscores
- You can create a branch from an issue in the Github UI. It will prefix the
  branch name with the issue number.
- If you create a branch manually, use one of these prefixes:
  - `feature/` - New features
  - `fix/` - Bug fixes
  - `build/` - Build system changes
  - `doc/` - Documentation updates
  - `chore/` - Miscellaneous tasks (e.g., refactoring, tooling)

**Example**: `feature/add-container-support` or `fix/memory-leak`

### Commit Style

We use [gitmoji](https://gitmoji.dev/) for commit messages. Consider using the
[VS Code Plugin](https://marketplace.visualstudio.com/items?itemName=seatonjiang.gitmoji-vscode)
to make this easier.

**Examples**:

- `✨ add container registry support`
- `🐛 fix memory leak in image builder`
- `📝 update installation guide`

## Creating Issues

Before creating a new issue, please search
[existing issues](https://github.com/kaweezle/iknite/issues) to avoid
duplicates.

### Bug Reports

Include the following information:

- **Title**: Concise description of the problem
- **Steps to reproduce**: Detailed steps to trigger the bug
- **Expected vs actual behavior**: What should happen and what does happen
- **Environment**: OS, Go version, Iknite version, relevant configuration
- **Additional context**: Screenshots, logs, or error messages

### Feature Requests

Describe:

- **Problem**: What problem does this solve?
- **Proposed solution**: Your suggested approach
- **Alternatives**: Other solutions you've considered
- **Use cases**: Examples or scenarios that illustrate the need

### Questions

For questions about using Iknite:

1. Check the [documentation](https://kaweezle.github.io/iknite/) first
2. Search existing issues and discussions
3. If still needed, open an issue with context and relevant code snippets

Consider using
[GitHub Discussions](https://github.com/kaweezle/iknite/discussions) for general
questions and community support.

#### Helpful Resources

- [GitHub: Creating an Issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/creating-an-issue)
- [How to Write a Good Bug Report](https://testlio.com/blog/the-ideal-bug-report/)
- [Mozilla: Bug Writing Guidelines](https://developer.mozilla.org/en-US/docs/Mozilla/QA/Bug_writing_guidelines)
- [RStudio: Writing Good Feature Requests](https://github.com/rstudio/rstudio/wiki/Writing-Good-Feature-Requests)

## Contribution Workflow

### Step-by-Step Process

1. **Fork and clone**:

   ```bash
   git clone https://github.com/YOUR-USERNAME/iknite.git
   cd iknite
   git remote add upstream https://github.com/kaweezle/iknite.git
   ```

2. **Create an issue** at
   [github.com/kaweezle/iknite/issues](https://github.com/kaweezle/iknite/issues)
   to track your contribution.

3. **Create a branch** from `main`:

   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Make your changes**:

   - Write code following the [code style guidelines](#code-style)
   - Add unit tests (aim for >85% coverage)
   - Test locally: `go test ./...`

5. **Prepare for submission**:

   ```bash
   # Fetch and rebase on latest main
   git fetch upstream
   git rebase upstream/main

   # Run pre-commit checks
   pre-commit run --all-files
   ```

6. **Push and create PR**:

   ```bash
   git push origin feature/your-feature-name
   ```

   - Create a pull request to `main`
   - Reference the issue (e.g., "Fixes #123")

7. **Respond to review**:

   - Address feedback promptly
   - Push additional commits as needed
   - Once approved, a maintainer will merge your PR

### Additional Resources

- [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
- [GitHub Flow](https://guides.github.com/introduction/flow/)
- [Fork and Pull Request Workflow](https://github.com/susam/gitpr)
- [Git Forks and Upstreams](https://www.atlassian.com/git/tutorials/git-forks-and-upstreams)

## Development Guidelines

### Commit Guidelines

**Key principles**:

- Keep commits focused and atomic
- Squash commits that fix issues introduced in the PR with the commit that
  introduced them
- Rebase to maintain a clean history
- Separate formatting changes from functional changes
- Remove empty commits from rebases

Learn about
[rebasing](https://www.atlassian.com/git/tutorials/rewriting-history/git-rebase)
and
[rewriting history](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History).

#### Commit Messages

Follow
[this guide](https://gist.github.com/robertpainsi/b632364184e70900af4ab688decf6f53)
for writing good commit messages. Keeping the first line under 50 characters is
recommended but not enforced. Keep it short but descriptive.

**Using gitmoji with Conventional Commits**:

- Replace conventional types (`feat:`, `fix:`) with emojis (e.g., `✨`, `🐛`)
- Link to issues with `#` (e.g., "Fixes #123")

### Code Style

**Follow the platform conventions**. For Go, reference:

- [Effective Go](https://go.dev/doc/effective_go)
- [The Go Style Guide](https://google.github.io/styleguide/go/)

**General guidelines**:

- ✅ Use consistent naming conventions (camelCase, snake_case, etc.)
- ✅ Write descriptive names - avoid abbreviations
- ✅ Fix typos immediately (project uses [CSpell](https://cspell.org/) with
  [pre-commit](https://pre-commit.com/))
- ✅ Use English for all identifiers
- ✅ Use verbs for methods, nouns for properties
- ✅ Keep methods small and focused
- ✅ Prefer loose coupling
- ✅ Fail fast and explicitly - never fail silently

### Testing

- Write unit tests for all changes
- Aim for >85% code coverage
- Run tests locally before submitting: `go test ./...`

### Versioning

We follow [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH).

## Documentation

We encourage documentation contributions! Documentation is in the `docs`
directory, written in
[Markdown](https://daringfireball.net/projects/markdown/syntax) and built with
[MkDocs](https://www.mkdocs.org/) using
[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/).

### Adding Documentation

Navigation is managed by
[awesome-nav](https://github.com/lukasgeiter/mkdocs-awesome-nav). Main
categories are defined in `docs/docs/.nav.yml`.

**For files outside `docs/docs`**, create symbolic links:

```bash
cd docs/docs/some/category
ln -s ../../../../path/to/your/README.md .
```

### Local Preview

!!! tip "Required Tools"

Python and [uv](https://github.com/astral-sh/uv)

Preview documentation while editing:

```bash
cd docs
uv run mkdocs serve --livereload
```

Then browse at [http://localhost:8000/](http://localhost:8000/). Stop with
`CTRL+C`.
