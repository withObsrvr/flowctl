# Pitch #0 Removal Audit - Detailed Findings

**Date**: 2025-10-26
**Appetite**: 3 days
**Status**: Ready for execution

## Executive Summary
Found **6 major categories** of broken/stubbed features totaling **~600+ LOC** to remove.

---

## 1. Nomad Generator (COMPLETE STUB)

### Files to Delete:
- `internal/generator/nomad.go` (28 lines)

### Current State:
```go
func (g *NomadGenerator) Generate(pipeline *model.Pipeline, opts model.TranslationOptions) ([]byte, error) {
    logger.Debug("Nomad generator not yet implemented")
    return nil, fmt.Errorf("Nomad generator not yet implemented")
}
```

### References to Update:
- `README.md` line 14: Remove "Nomad" from "Multi-platform deployment"
- `README.md` line 113: Remove "nomad" from supported formats
- Translation command will automatically fail with "unknown format"

---

## 2. Kubernetes Generator (UNTESTED, 602 LINES)

### Decision Point:
**Option A**: Delete entirely (~602 lines removed)
**Option B**: Mark as experimental with warning

### Files Affected:
- `internal/generator/kubernetes.go` (602 lines)

### Current State:
- Full implementation exists
- Generates Deployments, Services, ConfigMaps, Namespaces
- **BUT**: Zero tests, unknown if it works
- **Risk**: Presents as working but may be broken

### Recommendation:
**DELETE**. Reasons:
1. Completely untested
2. Claims on line 112 say "coming soon" but code exists
3. Better to remove than ship potentially broken code
4. Can be added back properly later with tests

### References to Update:
- `README.md` line 14: Remove "Kubernetes" from features
- `README.md` line 112: Remove kubernetes from formats

---

## 3. Get Command (PLACEHOLDER OUTPUT)

### Files to Delete:
- `cmd/get.go` (53 lines)

### Current State:
```go
// TODO: Fetch resources from runtime
// For now, just print a placeholder message
fmt.Printf("Listing %s resources...\n", kind)
```

### User Experience:
```bash
$ flowctl get pipelines
Listing Pipeline resources...
NAME    STATUS  AGE
# ^ FAKE OUTPUT, no actual data
```

### References to Update:
- `cmd/root.go`: Remove command registration
- `README.md`: No references to remove (good!)

---

## 4. Logs Command (PLACEHOLDER OUTPUT)

### Files to Delete:
- `cmd/logs.go` (77 lines)

### Current State:
```go
// TODO: Fetch logs from runtime
// For now, just print a placeholder message
fmt.Printf("Fetching logs for %s/%s...\n", resourceType, resourceName)
```

### User Experience:
```bash
$ flowctl logs pipeline/my-pipeline
Fetching logs for pipeline/my-pipeline...
# ^ FAKE OUTPUT, no actual logs
```

### References to Update:
- `cmd/root.go`: Remove command registration
- `README.md`: No references to remove (good!)

---

## 5. Bundled Runtime Download (STUB FUNCTIONS)

### Files to Modify:
- `internal/sandbox/runtime/runtime.go`

### Functions to Delete:
- `ensureBundledRuntime()` (lines 155-180) - 26 lines
- `downloadBundledRuntime()` (lines 182-190) - 9 lines

### Current State:
```go
func (r *Runtime) downloadBundledRuntime(binDir string) error {
    // TODO: This is a placeholder implementation
    logger.Info("Downloading bundled runtime binaries")
    return fmt.Errorf("bundled runtime download is not yet implemented. Please use --use-system-runtime flag")
}
```

### Changes Needed:
1. Remove both functions
2. Update `NewRuntime()` to always require system runtime
3. Make `--use-system-runtime` flag the default (or remove it)
4. Update error messages to clearly state Docker/nerdctl must be installed

### References to Update:
- `CLAUDE.md` line 13: Update to say runtime must be installed
- `cmd/sandbox/start.go`: Update help text

---

## 6. Commented CUE Validation Code (DEAD CODE)

### Files to Modify:
- `internal/translator/cue_validator.go`

### Lines to Delete:
- Lines 68-140: Commented out CUE implementation (73 lines)

### Current State:
```go
func (v *CUEValidator) Validate(pipeline *model.Pipeline) error {
    // TODO: Fix CUE schema loading - for now, use basic validator
    return basicValidator.Validate(pipeline)
}

// Everything below is commented out...
// func (v *CUEValidator) Validate...
// ... 140 lines of commented code
```

### Action:
1. Delete all commented code
2. Acknowledge that basic validation is the reality
3. Update documentation to say "YAML validation" not "CUE schema validation"

---

## Summary of Changes

### Files to DELETE:
1. `internal/generator/nomad.go` (28 LOC)
2. `internal/generator/kubernetes.go` (602 LOC) - **Decision needed**
3. `cmd/get.go` (53 LOC)
4. `cmd/logs.go` (77 LOC)

**Total if deleting kubernetes**: ~760 LOC deleted
**Total if keeping kubernetes**: ~158 LOC deleted

### Files to MODIFY:
1. `internal/sandbox/runtime/runtime.go` (remove 35 LOC)
2. `internal/translator/cue_validator.go` (remove 73 LOC commented)
3. `README.md` (update 3-5 lines)
4. `CLAUDE.md` (update 2-3 lines)
5. `cmd/root.go` (remove 2 command registrations)

### Total Impact:
- **Minimum removal**: ~266 LOC (if keeping kubernetes)
- **Maximum removal**: ~868 LOC (if deleting kubernetes)
- **Documentation updates**: ~10 lines across 2 files

---

## Implementation Order

### Day 1: Delete Stub Commands (2-3 hours)
1. ✅ Delete `cmd/get.go`
2. ✅ Delete `cmd/logs.go`
3. ✅ Update `cmd/root.go` to remove registrations
4. ✅ Test: `go build` succeeds
5. ✅ Test: `flowctl --help` doesn't show removed commands

### Day 2: Delete Generators & Clean Runtime (3-4 hours)
1. ✅ Delete `internal/generator/nomad.go`
2. ✅ **DECIDE**: Delete or keep `internal/generator/kubernetes.go`
3. ✅ Update `internal/sandbox/runtime/runtime.go`:
   - Remove `ensureBundledRuntime()`
   - Remove `downloadBundledRuntime()`
   - Update `NewRuntime()` logic
4. ✅ Delete commented code in `internal/translator/cue_validator.go`
5. ✅ Test: `go build` succeeds
6. ✅ Test: Sandbox still works with `--use-system-runtime`

### Day 3: Documentation & Polish (2-3 hours)
1. ✅ Update `README.md`:
   - Line 14: Remove Kubernetes/Nomad
   - Lines 112-113: Remove "coming soon" formats
2. ✅ Update `CLAUDE.md`:
   - Sandbox section (runtime requirement)
3. ✅ Run full test suite: `go test ./...`
4. ✅ Test all examples still work
5. ✅ Create changelog/commit message
6. ✅ Final build verification

---

## Decision Required: Kubernetes Generator

### Option A: DELETE (Recommended)
**Pros**:
- Honest about capabilities
- 602 lines removed
- No risk of shipping broken code
- Can add back properly with tests later

**Cons**:
- Loses potentially working code
- More removal work

### Option B: KEEP with Warning
**Pros**:
- Preserves work
- Might actually work

**Cons**:
- Untested code in production
- Still misleading to users
- Maintains technical debt

**Recommendation**: **DELETE**. Better to be honest.

---

## Success Criteria

After completion:
```bash
# Build succeeds
make build
# ✓ Success

# Help shows only working commands
./bin/flowctl --help
# ✓ No 'get' command
# ✓ No 'logs' command

# Translation rejects invalid formats
./bin/flowctl translate --format nomad examples/minimal.yaml
# ✗ Error: unknown format 'nomad'
# ✓ Suggests: Supported formats: docker-compose

# Sandbox requires system runtime
./bin/flowctl sandbox start --pipeline test.yaml
# ✗ Error: Please install Docker or nerdctl
# ✓ Clear instructions provided

# README is honest
grep "coming soon" README.md
# (no output)

# Tests pass
go test ./...
# ✓ All passing
```

---

## Risk Mitigation

### If Behind Schedule:
**Day 1 must-haves**:
- Delete get/logs commands
- Update README

**Day 2 can-defer**:
- Keep bundled runtime stubs (just document they don't work)
- Keep commented CUE code (remove later)

**Day 3 can-defer**:
- Detailed testing can be lighter
- Documentation can be minimal

### If Discover Issues:
- Each deletion is independent
- Can partially ship (e.g., remove commands but keep generators)
- Always have working build as escape hatch

---

## Questions for User

1. **Kubernetes generator**: Delete or keep?
2. **Cool-down after**: Need 1 day break, or proceed to Pitch #1 immediately?
3. **Commit strategy**: One big commit or incremental?

---

**Status**: Audit complete, ready to begin removal ✂️
