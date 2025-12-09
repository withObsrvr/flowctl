# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed

- **BREAKING:** Removed `flowctl apply` command - use `flowctl translate` instead
- **BREAKING:** Removed `flowctl new` command - use `flowctl init` instead
- **BREAKING:** Removed `flowctl context` command - feature was not implemented
- Removed redundant `flowctl help` command - use `--help` flag instead

### Why?

These commands were either incomplete stubs (`apply`), redundant (`new`, `help`),
or premature features (`context`). Removing them reduces user confusion,
eliminates false promises, and reduces maintenance burden by ~500 lines of code.

### Migration Guide

| Old Command | New Alternative |
|-------------|----------------|
| `flowctl apply -f file.yaml --translate-only` | `flowctl translate -f file.yaml` |
| `flowctl new pipeline foo` | `flowctl init` (interactive wizard) |
| `flowctl context --show` | Use `--context` flag on commands |
| `flowctl help <cmd>` | `flowctl <cmd> --help` |

### Changed

- Fixed linting issues in `flowctl init` command (redundant newlines in fmt.Println)

---

## Previous Releases

_No previous releases documented_
