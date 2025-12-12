# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.50.x  | :white_check_mark: |
| < 0.50  | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in CLASP, please report it responsibly.

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Email security concerns to the repository maintainers via GitHub's private vulnerability reporting feature
3. Or, use GitHub's Security Advisory feature: https://github.com/jedarden/CLASP/security/advisories/new

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Resolution Target**: Within 30 days for critical issues

## Security Measures

CLASP implements several security best practices:

### Secret Management
- API keys are masked in logs using `MaskAPIKey()`, `MaskAllSecrets()`, and `MaskJSONSecrets()`
- No secrets are stored in plain text
- Environment variables are the recommended method for API key configuration

### Authentication
- Optional API key authentication for the proxy endpoint
- Timing-attack resistant key comparison using `subtle.ConstantTimeCompare()`
- Configurable anonymous access for health and metrics endpoints

### Rate Limiting
- Configurable rate limiting to prevent abuse
- Token bucket algorithm with burst support

### Input Validation
- Request validation for required fields
- JSON schema validation for API requests

### Dependencies
- Dependabot enabled for automatic security updates
- Regular dependency audits via `go mod tidy` and `npm audit`

## Security Configuration

### Production Recommendations

```bash
# Enable authentication
export AUTH_ENABLED=true
export AUTH_API_KEY=your-secure-api-key

# Enable rate limiting
export RATE_LIMIT_ENABLED=true
export RATE_LIMIT_REQUESTS=60
export RATE_LIMIT_WINDOW=60

# Enable circuit breaker
export CIRCUIT_BREAKER_ENABLED=true
```

### Security Warnings

CLASP will log warnings at startup if:
- Authentication is disabled
- Rate limiting is disabled

## Automated Security Scanning

This project uses:
- **gitleaks**: Secret detection in commits
- **gosec**: Go security scanner
- **Dependabot**: Automated dependency updates
- **CodeQL**: GitHub's semantic code analysis (planned)

## Disclosure Policy

- Security vulnerabilities will be disclosed after a fix is available
- Credit will be given to reporters unless they prefer anonymity
- We follow coordinated disclosure practices
