# Shape Up Pitches for flowctl

This directory contains **6 shaped pitches** for the next development cycles, based on analysis of the codebase. Each pitch follows the Shape Up methodology with fixed time budgets and variable scope.

## 📊 Pitch Overview

| # | Pitch | Appetite | Impact | Status |
|---|-------|----------|--------|--------|
| 0 | [Remove Broken Features](00-remove-broken-features.md) | 3 days | 🟣 Foundation | Not Started |
| 1 | [Docker Driver Deployment](01-docker-driver-deployment.md) | 2 weeks | 🔴 Critical | Not Started |
| 2 | [Pipeline Error Handling](02-pipeline-error-handling.md) | 1 week | 🔴 Critical | Not Started |
| 3 | [Real CUE Validation](03-real-cue-validation.md) | 1 week | 🟡 Important | Not Started |
| 4 | [CLI Backend (Get/Logs)](04-cli-backend-get-logs.md) | 1 week | 🟡 Important | Not Started |
| 5 | [Integration Test Foundation](05-integration-test-foundation.md) | 2 weeks | 🟢 Foundation | Not Started |
| 6 | [Bundled Runtime Download](06-bundled-runtime-download.md) | 1 week | 🟡 Important | Not Started |

## 🎯 Recommended Prioritization

### 🆕 Option A: Start with Cleanup (Recommended)
Start by removing broken features, then build on solid foundation:

**Cycle 0** (3 days): **Remove Broken Features** ← Make tool honest
**Cool-down** (1 day)
**Cycle 1** (2 weeks): **Docker Driver Deployment** ← Core functionality
**Cool-down** (3-4 days)
**Cycle 2** (1 week): **Error Handling** ← Stability
... continue with remaining pitches

### Option B: Start with Implementation
Skip cleanup, focus on building features:

### Cycle 1 (2 weeks): Foundation
**Pitch #1: Docker Driver Deployment**
- **Why first**: Nothing actually deploys right now. This is the core value prop.
- **Risk**: Highest complexity, needs time
- **Unlock**: Makes the tool actually useful

### Cycle 2 (1 week): Stability
**Pitch #2: Pipeline Error Handling**
- **Why second**: Once deployment works, need errors visible
- **Risk**: Critical bugs hidden by silent failures
- **Unlock**: Production-ready error visibility

### Cycle 3 (1 week): Quality
**Pitch #3: Real CUE Validation** OR **Pitch #4: CLI Backend**
- **Option A (CUE)**: If quality/correctness is priority
- **Option B (CLI)**: If debugging/operations is priority
- Both improve DX significantly

### Cycle 4 (2 weeks): Confidence
**Pitch #5: Integration Test Foundation**
- **Why later**: Need working features to test first
- **Why important**: Prevents regression as features grow
- **Unlock**: Confidence to ship fast

### Cycle 5 (1 week): Polish
**Pitch #6: Bundled Runtime** OR **Remaining from Cycle 3**
- Nice-to-have improvements
- Can be deferred if higher priorities emerge

## 📋 What Each Pitch Solves

### 0️⃣ Remove Broken Features ⭐ NEW
**Current state**: Tool has 6+ features that don't work (stubs, TODOs)
**After**: Smaller tool that's 100% honest about capabilities
**Files affected**: Delete ~590 LOC across 6 files
**Impact**: Users trust the tool—everything that exists actually works

### 1️⃣ Docker Driver Deployment
**Current state**: `flowctl apply` translates YAML but doesn't start anything
**After**: Pipelines actually deploy and run with a single command
**Files affected**: `internal/drivers/docker/docker.go` (currently all TODOs)

### 2️⃣ Pipeline Error Handling
**Current state**: Errors silently ignored, data lost, no visibility
**After**: Clear error messages, retry logic, failures stop pipelines
**Files affected**: `internal/core/pipeline.go` (3 TODO comments for error handling)

### 3️⃣ Real CUE Validation
**Current state**: CUE validator disabled, 140 lines commented out
**After**: Strong schema validation catches errors before deployment
**Files affected**: `internal/translator/cue_validator.go` (just uncomment + fix)

### 4️⃣ CLI Backend (Get/Logs)
**Current state**: Commands exist but return fake placeholder output
**After**: Real data from control plane, actual container logs
**Files affected**: `cmd/get.go`, `cmd/logs.go` (both stubs)

### 5️⃣ Integration Test Foundation
**Current state**: Only unit tests, no end-to-end verification
**After**: E2E tests, integration tests, example validation
**Files affected**: New `test/` directory structure

### 6️⃣ Bundled Runtime Download
**Current state**: Must use `--use-system-runtime` flag, download stubbed
**After**: Runtime downloads automatically, flag optional
**Files affected**: `internal/sandbox/runtime/runtime.go` (download funcs stubbed)

## 🎲 Alternative Sequences

### Sequence A: "Make it Work"
Prioritize getting basic functionality solid:
1. Docker Driver (2w)
2. Error Handling (1w)
3. Integration Tests (2w)
4. **Cool-down**
5. CUE Validation (1w)
6. CLI Backend (1w)

### Sequence B: "Developer Experience"
Prioritize DX and tooling:
1. Docker Driver (2w)
2. CLI Backend (1w)
3. CUE Validation (1w)
4. **Cool-down**
5. Error Handling (1w)
6. Integration Tests (2w)

### Sequence C: "Quality First"
Prioritize correctness and testing:
1. Integration Tests (2w) ← Build test foundation first
2. Docker Driver (2w)
3. **Cool-down**
4. Error Handling (1w)
5. CUE Validation (1w)
6. CLI Backend (1w)

## 🚨 Critical Issues Found

All these pitches address **actual broken code** in the current codebase:

| Issue | Severity | Files Affected |
|-------|----------|----------------|
| Docker driver doesn't deploy | 🔴 Critical | `internal/drivers/docker/docker.go` |
| Errors silently ignored | 🔴 Critical | `internal/core/pipeline.go` |
| CUE validation disabled | 🟡 High | `internal/translator/cue_validator.go` |
| CLI commands are stubs | 🟡 High | `cmd/get.go`, `cmd/logs.go` |
| No integration tests | 🟡 High | Entire codebase |
| Runtime download broken | 🟠 Medium | `internal/sandbox/runtime/runtime.go` |

## 📐 Shape Up Principles Applied

Each pitch includes:
- ✅ **Appetite** (fixed time): 1 week or 2 weeks
- ✅ **Problem statement**: Clear user pain
- ✅ **Solution sketch**: Fat-marker level (not detailed specs)
- ✅ **Rabbit holes**: What NOT to do
- ✅ **No-gos**: Explicitly out of scope
- ✅ **Done criteria**: Concrete success example
- ✅ **Scope line**: MUST/NICE/COULD tiers

### Cutting Scope
Every pitch has a clear scope line. If running behind:
1. Cut "COULD HAVE" items first
2. Then "NICE TO HAVE" items
3. Ship MUST HAVE items on deadline

### Cool-downs
**Mandatory between cycles**:
- 2-3 days after 1-week cycles
- 4-5 days after 2-week cycles
- Use for: bug fixes, refactoring, exploration, rest

## 🎯 Getting Started

1. **Review pitches** - Read through each one
2. **Pick one** - Choose based on priorities
3. **Get approval** - Confirm scope and approach
4. **Track on hill** - Use hill chart for progress
5. **Ship on deadline** - Cut scope if needed, don't extend time

## 📚 Additional Context

All findings based on automated codebase exploration that found:
- 13+ TODO/FIXME comments
- 15+ placeholder implementations
- 3+ major unimplemented features in docs
- 183 uses of `interface{}` (type safety opportunities)
- Only 10 test files (limited coverage)

See [exploration results](../docs/codebase-analysis.md) for full audit.

---

**Next step**: Pick a pitch and start shaping! 🚀
