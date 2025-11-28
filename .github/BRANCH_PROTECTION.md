# Branch Protection Setup

This document describes how to configure branch protection rules in GitHub to enforce quality gates.

## Required Status Checks

To enable the PR checks we've configured, you need to set up branch protection rules in GitHub.

### Steps to Configure

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Branches**
3. Click **Add rule** (or edit existing rule for `main` branch)
4. Configure the following settings:

### Required Settings

#### Branch Name Pattern
```
main
```

#### Protection Rules

✅ **Require a pull request before merging**
- ✅ Require approvals: 1 (recommended)
- ✅ Dismiss stale pull request approvals when new commits are pushed

✅ **Require status checks to pass before merging**
- ✅ Require branches to be up to date before merging
- **Required status checks:**
  - `Run Tests` (from test.yml)
  - `Require Tests to Pass` (from pr-checks.yml)
  - `Security Checks` (from pr-checks.yml)
  - `Build Verification` (from pr-checks.yml)
  - `Code Quality` (from pr-checks.yml)
  - `Lint` (from test.yml)

✅ **Require conversation resolution before merging**
- Ensures all PR comments are addressed

✅ **Do not allow bypassing the above settings**
- Applies to administrators too (recommended for production)

### Optional but Recommended

- ✅ Require linear history (prevents merge commits)
- ✅ Require deployments to succeed before merging
- ✅ Lock branch (for production branches)

## Workflow Files

We've created two workflow files:

### 1. `test.yml` - Continuous Integration
Runs on: `push` and `pull_request` to `main` and `develop` branches

**Jobs:**
- **test**: Runs all tests with race detector and coverage
- **lint**: Runs golangci-lint for code quality

### 2. `pr-checks.yml` - Pull Request Quality Gates
Runs on: `pull_request` events (opened, synchronized, reopened)

**Jobs:**
- **require-tests**: Runs tests with race detector and checks coverage threshold (10%)
- **security**: Runs Gosec security scanner
- **build**: Verifies builds for Linux, macOS, and Windows
- **quality**: Runs go vet, checks formatting, and runs staticcheck

## Coverage Requirements

Current minimum coverage threshold: **10%**

To increase the threshold, edit `.github/workflows/pr-checks.yml`:

```yaml
# Set minimum coverage threshold (currently lenient, increase over time)
threshold=10  # Change this value
```

Recommended progression:
- Phase 1: 10% (current)
- Phase 2: 30% (after P2 tests)
- Phase 3: 50% (after P3 tests)
- Phase 4: 70% (production-ready)

## Testing Locally Before Pushing

Before creating a PR, run these commands locally:

```bash
# Run tests with race detector
go test -race ./...

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Run linter
golangci-lint run

# Check formatting
gofmt -l .

# Run go vet
go vet ./...

# Run security scanner
gosec ./...
```

## Troubleshooting

### Tests Fail in CI but Pass Locally

1. Check if you have uncommitted changes
2. Ensure `go.mod` and `go.sum` are up to date: `go mod tidy`
3. Check for race conditions: `go test -race ./...`

### Coverage Below Threshold

Add tests for your new code. Focus on:
1. Handler tests (highest impact)
2. Service tests (business logic)
3. Repository tests (data access)

### Linter Failures

Run locally: `golangci-lint run --fix`

### Format Failures

Run: `gofmt -w .`

## CI/CD Best Practices

1. **Keep tests fast**: Target < 5 minutes total execution time
2. **Fail fast**: Run quick checks (formatting, vet) before slow tests
3. **Parallel execution**: Tests run with `-race` flag for concurrent safety
4. **Caching**: Go modules and build cache are enabled for faster runs
5. **Security**: Gosec scans for common security issues

## Secrets Configuration

For full functionality, add these secrets to GitHub:

1. Go to **Settings** → **Secrets and variables** → **Actions**
2. Add the following secrets:
   - `CODECOV_TOKEN` - For coverage reporting (optional)

## Monitoring

- View workflow runs: **Actions** tab in GitHub
- Check status badges: Add to README.md
- Review coverage trends: Enable Codecov integration

## Questions?

If tests fail unexpectedly:
1. Check the Actions tab for detailed logs
2. Compare local vs CI environment
3. Ensure all dependencies are in `go.mod`
4. Check for environment-specific issues
