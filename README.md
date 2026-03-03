# cispectre

[![CI](https://github.com/ppiankov/cispectre/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/cispectre/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ppiankov/cispectre)](https://goreportcard.com/report/github.com/ppiankov/cispectre)

**cispectre** — GitHub Actions waste and hygiene auditor. Part of [SpectreHub](https://github.com/ppiankov/spectrehub).

## What it is

- Scans GitHub Actions workflows for waste: long runners, redundant jobs, stale workflows
- Identifies unused secrets and oversized artifacts
- Estimates wasted CI minutes
- Outputs text, JSON, and SpectreHub formats

## What it is NOT

- Not a CI/CD platform — audits existing workflows
- Not a remediation tool — reports only, never modifies workflows
- Not a security scanner — checks efficiency, not supply chain attacks

## Quick start

### Homebrew

```sh
brew tap ppiankov/tap
brew install cispectre
```

### From source

```sh
git clone https://github.com/ppiankov/cispectre.git
cd cispectre
make build
```

### Usage

```sh
cispectre scan --org ppiankov --format json
```

## CLI commands

| Command | Description |
|---------|-------------|
| `cispectre scan` | Audit GitHub Actions workflows for waste |
| `cispectre version` | Print version |

## SpectreHub integration

cispectre feeds CI/CD waste findings into [SpectreHub](https://github.com/ppiankov/spectrehub) for unified visibility across your infrastructure.

```sh
spectrehub collect --tool cispectre
```

## Safety

cispectre operates in **read-only mode**. It inspects and reports — never modifies, deletes, or alters your workflows.

## License

MIT — see [LICENSE](LICENSE).

---

Built by [Obsta Labs](https://github.com/ppiankov)
