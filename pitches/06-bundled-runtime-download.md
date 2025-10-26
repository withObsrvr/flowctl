# Pitch: Make Bundled Runtime Download Actually Work

## Problem
The `--use-system-runtime` flag is **required** to use the sandbox because bundled runtime download is stubbed out:

`internal/sandbox/runtime/runtime.go` lines 156-189:
```go
func (r *RuntimeManager) ensureBundledRuntime() error {
    return fmt.Errorf("bundled runtime not yet implemented")
}

func (r *RuntimeManager) downloadBundledRuntime() error {
    return fmt.Errorf("download not implemented")
}
```

**User pain**:
- Must manually install Docker/nerdctl
- Flag requirement is confusing
- Breaks the "it just works" promise
- NixOS users especially affected (binary path issues)

**Documentation says** bundled runtime should work but doesn't.

## Appetite
**1 week** - Feature is already sketched out, just needs implementation.

## Solution (Fat-marker sketch)

### Two Approaches (Pick One Based on Complexity)

#### Approach A: Download nerdctl + containerd (Ambitious)
```
Check if bundled runtime exists
  ↓
Download nerdctl binary from GitHub releases
  ↓
Download containerd binary
  ↓
Extract to ~/.flowctl/runtime/
  ↓
Start containerd daemon (rootless mode)
  ↓
Use nerdctl for container operations
```

**Pros**: Truly standalone, no dependencies
**Cons**: Complex, daemon management, rootless setup tricky
**Estimate**: 4-5 days, high risk

#### Approach B: Download nerdctl, require Docker (Pragmatic)
```
Check for Docker daemon running
  ↓
Download nerdctl binary only
  ↓
Extract to ~/.flowctl/runtime/bin/
  ↓
Use nerdctl CLI with existing Docker daemon
```

**Pros**: Much simpler, leverages existing Docker
**Cons**: Still requires Docker installed
**Estimate**: 2 days, low risk

### Recommended: Start with Approach B
- Ship working solution fast
- Can upgrade to Approach A later if needed
- Most users already have Docker
- Focus is on **not requiring the flag**, not true isolation

### Implementation Steps (Approach B)
1. Detect OS and architecture
2. Construct GitHub release URL for nerdctl
3. Download binary (with progress bar)
4. Verify checksum
5. Extract and set executable permissions
6. Update RuntimeManager to use bundled nerdctl

## Rabbit Holes to Avoid
- ❌ **Don't build a custom container runtime** - Download existing binaries
- ❌ **Don't support every OS/arch combination** - Linux amd64/arm64 + macOS only
- ❌ **Don't implement rootless containerd from scratch** - Hard, ship simpler version first
- ❌ **Don't add auto-update of runtime** - Download once, manual update fine
- ❌ **Don't verify signatures cryptographically** - Checksum is enough

## No-Gos (Explicitly Out of Scope)
- Windows support (Linux/macOS only for now)
- Custom container runtime builds
- Containerd daemon management (Approach A)
- Rootless containerd setup
- Runtime version management (multiple versions)
- Runtime auto-updates
- Proxy/corporate firewall support for downloads

## Done (Concrete Success Criteria)

### Must Demo This:

**Scenario 1: First Run (No Bundled Runtime)**
```bash
# Delete any existing bundled runtime
rm -rf ~/.flowctl/runtime

# Start sandbox WITHOUT --use-system-runtime flag
./bin/flowctl sandbox start --pipeline examples/sandbox-pipeline.yaml

# Output:
# → Bundled runtime not found
# → Downloading nerdctl v1.7.0 for linux/amd64...
# [========================================] 100%
# → Extracted to ~/.flowctl/runtime/bin/nerdctl
# → Verifying installation...
# ✓ Runtime ready
# → Starting sandbox...
# ✓ Sandbox started successfully
```

**Scenario 2: Subsequent Runs (Runtime Exists)**
```bash
./bin/flowctl sandbox start --pipeline examples/sandbox-pipeline.yaml

# Output:
# → Using bundled runtime: nerdctl v1.7.0
# → Starting sandbox...
# ✓ Sandbox started successfully
#
# (No download, uses existing)
```

**Scenario 3: System Runtime Still Works**
```bash
./bin/flowctl sandbox start --use-system-runtime --pipeline examples/sandbox-pipeline.yaml

# ✓ Uses system Docker/nerdctl as before
# ✓ Flag still supported for users who prefer it
```

### Code Changes Expected:
- `internal/sandbox/runtime/runtime.go` - Implement download logic (150-200 LOC)
- `internal/sandbox/runtime/download.go` - HTTP download with progress (100 LOC)
- `internal/sandbox/runtime/verify.go` - Checksum verification (50 LOC)
- Tests for download and extraction (100 LOC)
- Update docs to reflect flag is optional (10 LOC)

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ Download nerdctl binary from GitHub releases
✓ Detect OS and architecture (Linux/macOS, amd64/arm64)
✓ Extract binary to ~/.flowctl/runtime/bin/
✓ Set executable permissions
✓ Checksum verification
✓ Use bundled runtime by default
✓ --use-system-runtime flag still works
✓ Clear error if Docker daemon not running

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ Progress bar during download
○ Retry on download failure
○ Version checking (warn if outdated)
○ Manual runtime path via --runtime-path flag
○ Download resumption if interrupted

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ Containerd bundling (Approach A)
○ Rootless mode support
○ Multiple runtime versions
○ Runtime auto-update
○ Offline mode (pre-downloaded runtime)
```

## Technical Details

### nerdctl Release URL Pattern
```
https://github.com/containerd/nerdctl/releases/download/v{version}/nerdctl-{version}-{os}-{arch}.tar.gz
```

Example: `https://github.com/containerd/nerdctl/releases/download/v1.7.0/nerdctl-1.7.0-linux-amd64.tar.gz`

### Download Path
```
~/.flowctl/
  └── runtime/
      ├── bin/
      │   └── nerdctl
      └── cache/
          └── nerdctl-1.7.0-linux-amd64.tar.gz
```

### OS/Arch Detection
```go
goos := runtime.GOOS      // "linux", "darwin"
goarch := runtime.GOARCH  // "amd64", "arm64"
```

## Risk Mitigation
If download proves complex (firewalls, proxies, etc.):
- **Escape hatch**: Keep `--use-system-runtime` flag as workaround
- **Partial win**: Document manual download steps
- Still improves DX over current state

## Hill Progress Indicators

**Left side (figuring it out):**
- Can detect OS and architecture reliably
- Can construct correct download URL
- Can download file successfully
- Can extract tar.gz and get binary

**Right side (making it happen):**
- Download implemented with progress
- Checksum verification working
- RuntimeManager uses bundled binary
- Tests passing
- Works on Linux and macOS
- Documentation updated
