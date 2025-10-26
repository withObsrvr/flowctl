# Pitch: Remove Broken Features and Broken Promises

## Problem
**The codebase is full of features that don't work**:
- Nomad generator returns "not implemented"
- `get` and `logs` commands return fake output
- CUE validation is disabled (140 lines commented out)
- Docker driver methods are stubs
- Bundled runtime download doesn't work

**User experience**: Tool feels **half-baked and broken**. Commands exist but fail. Features promised but don't work. Users lose trust.

**Better alternative**: Ship a **smaller tool that actually works** than a big tool full of broken promises.

## Appetite
**3 days** - This is about deleting code and updating docs, not building features.

## Solution (Fat-marker sketch)

### Philosophy: Honest Capabilities
```
Before:
  ✗ 10 features, 5 work, 5 broken
  → Users confused, frustrated

After:
  ✓ 5 features, 5 work, 0 broken
  → Users confident, trust tool
```

### What to Remove

#### 🔴 REMOVE: Nomad Generator (Complete Stub)
**File**: `internal/generator/nomad.go`
- **Status**: Literally just returns "not implemented"
- **Impact**: Nobody can use this anyway
- **Action**: Delete file entirely
- **Update**: Remove from README's "supported formats" list

```bash
# Current (broken):
flowctl translate --format nomad pipeline.yaml
# Error: nomad format not yet implemented

# After removal:
flowctl translate --format nomad pipeline.yaml
# Error: unknown format 'nomad'. Supported: docker-compose, kubernetes
```

#### 🟡 REMOVE: Kubernetes Generator (Untested)
**File**: `internal/generator/kubernetes.go`
- **Status**: Code exists but completely untested
- **Impact**: Might work, might not—no one knows
- **Action**: Delete or mark as experimental
- **Alternative**: Add `--experimental` flag if keeping

#### 🟡 REMOVE: `get` and `logs` Commands (Placeholder Output)
**Files**: `cmd/get.go`, `cmd/logs.go`
- **Status**: Return fake placeholder text
- **Impact**: Misleading—looks like it works but doesn't
- **Action**: Remove commands entirely OR hide behind `--experimental` flag
- **Rationale**: No command is better than broken command

```bash
# Current (broken):
flowctl get pipelines
# Output: "Listing Pipeline resources..." (fake)

# After removal:
flowctl get pipelines
# Error: unknown command "get". See 'flowctl --help'
```

#### 🟢 KEEP BUT FIX: Docker Driver
**Why keep**: Core functionality—this is what the tool does
**Action**: Either implement (Pitch #1) or remove driver abstraction entirely
**Decision point**: If keeping, it's the top priority to implement

#### 🟡 REMOVE: Bundled Runtime Download
**File**: `internal/sandbox/runtime/runtime.go` (lines 156-189)
- **Status**: Returns "not implemented"
- **Impact**: Forces `--use-system-runtime` flag anyway
- **Action**: Remove download functions, require system runtime
- **Update**: Docs and help text to say "requires Docker/nerdctl installed"

```go
// Remove these stubbed functions:
func (r *RuntimeManager) ensureBundledRuntime() error {
    return fmt.Errorf("bundled runtime not yet implemented")
}

func (r *RuntimeManager) downloadBundledRuntime() error {
    return fmt.Errorf("download not implemented")
}

// Replace with:
// (nothing - just require --use-system-runtime)
```

#### 🟡 SIMPLIFY: CUE Validation → Basic Validation Only
**File**: `internal/translator/cue_validator.go`
- **Status**: 140 lines commented out, already using basic validator
- **Action**: Delete commented code, acknowledge basic validation is the reality
- **Update**: Docs to say "basic YAML validation" not "CUE schema validation"

### Documentation Updates
After removing features, update:
- **README.md**: Remove "coming soon" promises
- **CLAUDE.md**: Remove references to removed features
- **Help text**: Don't mention removed commands
- **Error messages**: Suggest what actually works

## Rabbit Holes to Avoid
- ❌ **Don't archive removed code in repo** - Git history is enough
- ❌ **Don't build feature flags for everything** - Just remove it
- ❌ **Don't leave "Coming soon!" comments** - Ship reality, not promises
- ❌ **Don't feel bad about cutting** - Smaller + working > bigger + broken

## No-Gos (Explicitly Out of Scope)
- Implementing any of these features (that's separate pitches)
- Migrating existing users (there aren't any yet)
- Deprecation period (nothing works, can't break what doesn't exist)
- Feature registry/plugin system for future additions

## Done (Concrete Success Criteria)

### Must Demo This:

**Scenario 1: Try to Use Removed Feature**
```bash
# Try removed nomad generator
flowctl translate --format nomad pipeline.yaml

# Before:
# Error: nomad format not yet implemented  ← Broken promise

# After:
# Error: unknown format 'nomad'
# Supported formats: docker-compose
# See 'flowctl translate --help' for details  ← Honest reality
```

**Scenario 2: Help Output is Honest**
```bash
flowctl --help

# Before (dishonest):
# Commands:
#   apply       Apply a pipeline
#   get         List resources       ← Doesn't work!
#   logs        Fetch logs           ← Doesn't work!
#   translate   Translate pipeline

# After (honest):
# Commands:
#   apply       Apply a pipeline
#   translate   Translate pipeline to deployment format
#   sandbox     Manage sandbox environment
#   server      Run control plane server
```

**Scenario 3: README Reflects Reality**
```markdown
# Before:
## Supported Deployment Targets
- ✅ Docker Compose
- 🚧 Kubernetes (coming soon)
- 🚧 Nomad (coming soon)
- 🚧 Local processes (coming soon)

# After:
## Supported Deployment Targets
- ✅ Docker Compose

Future formats may be added based on user demand.
```

**Scenario 4: Codebase is Cleaner**
```bash
# Count TODO comments
grep -r "TODO" internal/ | wc -l

# Before: 13 TODOs
# After: 5 TODOs (only for real features)

# Deleted files:
git log --oneline --diff-filter=D

# - Deleted internal/generator/nomad.go
# - Deleted cmd/get.go
# - Deleted cmd/logs.go
# - Removed stubbed runtime download functions
# - Deleted commented CUE validation code
```

### Files to Change:
- `internal/generator/nomad.go` - **DELETE** (30 LOC removed)
- `internal/generator/kubernetes.go` - **DELETE** or add experimental flag (200 LOC removed)
- `cmd/get.go` - **DELETE** (50 LOC removed)
- `cmd/logs.go` - **DELETE** (80 LOC removed)
- `internal/sandbox/runtime/runtime.go` - Remove stub functions (50 LOC removed)
- `internal/translator/cue_validator.go` - Delete commented code (140 LOC removed)
- `README.md` - Update to reflect reality (20 LOC changed)
- `CLAUDE.md` - Remove references to removed features (10 LOC changed)
- `cmd/root.go` - Remove command registrations (10 LOC changed)

**Total**: ~590 lines of code **removed** ✂️

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ Delete Nomad generator entirely
✓ Delete get and logs commands entirely
✓ Remove bundled runtime download stubs
✓ Delete commented CUE validation code
✓ Update README to remove "coming soon" promises
✓ Update help text to show only working commands
✓ Update CLAUDE.md to remove removed features

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ Delete Kubernetes generator (or mark experimental)
○ Add CHANGELOG entry explaining removals
○ Update examples to not reference removed features
○ Add architecture doc on "what flowctl does"

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ Blog post on "why we removed features"
○ Migration guide (if anyone is using this)
○ Feature request template for adding back
○ Roadmap doc for future additions
```

## Why This Makes Sense

### Benefits
1. **Users trust the tool** - Everything that exists, works
2. **Cleaner codebase** - 590 lines removed, easier to understand
3. **Faster development** - Don't maintain dead code
4. **Clear value prop** - "Docker Compose generator for pipelines" (focused!)
5. **Can add back later** - Git history preserves removed code
6. **Less confusion** - No more "Why doesn't this work?"

### Shape Up Principles
- **Ship working software** - Small but works > big but broken
- **Fixed time, variable scope** - Sometimes scope is "nothing"
- **Be honest** - Don't promise what you can't deliver
- **Focus** - Do fewer things, but do them well

## Alternative: Keep But Hide

Instead of deleting, could add `--experimental` flag:
```bash
flowctl --experimental get pipelines
# Works but warns: "⚠️  Experimental feature - may not work"
```

**Recommendation**: Delete instead of hide. Experimental flags are technical debt.

## What Stays vs Goes

### ✅ KEEP (Core Value)
- Docker Compose generation (works!)
- Sandbox environment (works!)
- Control plane server (works!)
- Pipeline translation (works!)
- Component model and DAG (works!)

### ✂️ REMOVE (Broken Promises)
- Nomad generator (stub)
- Kubernetes generator (untested)
- `get` command (fake output)
- `logs` command (fake output)
- Bundled runtime download (stub)
- Commented CUE validation code (dead code)

## Decision Tree

```
For each stub/broken feature:
  ↓
Is it core to the tool's value?
  YES → Either implement (Pitch #1-6) or it's a blocker
  NO → Continue
  ↓
Is anyone using it?
  YES → Deprecate gracefully
  NO → Continue
  ↓
Is it quick to fix? (<1 day)
  YES → Fix it
  NO → Continue
  ↓
DELETE IT
```

## Hill Progress Indicators

**Left side (figuring it out):**
- Decided what to keep vs remove
- Grepped for all references to removed features
- Identified docs that need updating

**Right side (making it happening):**
- Files deleted
- References removed
- Docs updated
- Help text accurate
- Tests passing (some removed)
- README reflects reality

## Ship Criteria

Ready to ship when:
```bash
# All removed commands fail cleanly
flowctl get pipelines
# ✗ Error: unknown command

# Help only shows working commands
flowctl --help | grep "get"
# (no output)

# README is honest
grep "coming soon" README.md
# (no output)

# Build succeeds
make build
# ✓ Success

# Remaining features work
./bin/flowctl translate examples/minimal.yaml
# ✓ Generated docker-compose.yml
```

---

## Summary

**Instead of building 6 broken features, ship 1 tool that works.**

This pitch is the **fastest path to a trustworthy tool**. Takes 3 days. Removes ~590 lines. Makes flowctl honest about what it does.

After this, can selectively add features back (via other pitches) when ready to actually implement them properly.

**Mantra: Working and honest > Big and broken.**
