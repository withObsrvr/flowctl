# flowctl Command Comparison: Current vs DX-First Reboot

## Executive Summary

The current flowctl implementation has a **foundational CLI structure** that partially aligns with the proposed DX-first reboot, but **lacks key developer experience features** and has some architectural misalignments that would require significant refactoring to achieve the "minutes-to-first-event, hours-to-prod" goal.

## Current Command Structure vs Proposed

### ✅ **Commands That Align Well**

| Current Command | Proposed Command | Alignment | Status |
|----------------|------------------|-----------|---------|
| `flowctl init` | `flowctl init` | 🟢 **Excellent** | Current creates workspace structure, proposal wants component scaffolding |
| `flowctl run` | `flowctl run` | 🟢 **Good** | Current runs pipelines, proposal wants one-shot CI/backfill execution |
| `flowctl apply` | `flowctl apply` | 🟢 **Good** | Current has basic apply, proposal wants diff & deploy |
| `flowctl logs` | `flowctl logs` | 🟢 **Good** | Current has basic log viewing, proposal wants streaming + historical |
| `flowctl version` | `flowctl version` | 🟢 **Perfect** | Direct match |

### ⚠️ **Commands That Need Enhancement**

| Current Command | Proposed Command | Gap Analysis | Required Changes |
|----------------|------------------|--------------|------------------|
| `flowctl list` | `flowctl status` | Current lists resources, proposal wants health/metrics | Add health checks, lag metrics, component status |
| Missing | `flowctl dev` | **🔴 Critical Gap** | Need live-reload, TUI logs, auto-rebuild - this is core DX |
| Missing | `flowctl add` | **🔴 Critical Gap** | Need component marketplace, template fetching, version pinning |

### 🔄 **Commands That Need Rethinking**

| Current Command | Purpose | DX-First Assessment | Recommendation |
|----------------|---------|---------------------|----------------|
| `flowctl server` | Runs control plane server | ❌ **Anti-pattern** for DX | Embed in `dev`/`apply`, hide complexity |
| `flowctl translate` | Converts to deployment formats | ❌ **Implementation detail** | Hide behind `apply`, not user-facing |
| `flowctl sandbox` | Local development environment | ⚠️ **Redundant** with proposed `dev` | Merge concepts into `flowctl dev` |

### ❌ **Missing Critical DX Commands**

| Proposed Command | Priority | Impact | Implementation Effort |
|------------------|----------|--------|---------------------|
| `flowctl dev` | 🟢 **MVP Critical** | **High** - Core developer loop | **High** - Needs process orchestrator, file watcher, TUI |
| `flowctl add` | 🟢 **MVP Critical** | **High** - Component discovery/reuse | **Medium** - Need component registry/marketplace |
| `flowctl rollback` | 🟡 **Stretch** | **Medium** - Production safety | **Medium** - Need deployment versioning |
| `flowctl package` | 🟡 **Stretch** | **Medium** - Sharing/distribution | **High** - Need OCI-style bundling |
| `flowctl completion` | 🟢 **MVP Critical** | **Low** - Polish but expected | **Low** - Standard Cobra feature |

## Detailed Command Analysis

### 1. `flowctl init` - Good Foundation, Needs Enhancement

**Current Implementation:**
```bash
flowctl init my-workspace
# Creates: flowctl.yaml, pipelines/, sources/, processors/, sinks/
```

**Proposed Enhancement:**
```bash
flowctl init my-pipeline
# Should create: flow.yml, components/, .gitignore
# Should include: working example pipeline, README, CI templates
```

**Gap:** Current creates workspace structure but lacks runnable example and CI templates.

### 2. `flowctl dev` - Critical Missing Command

**What We Need:**
```bash
flowctl dev
# Should provide: hot-reload, TUI logs, auto-rebuild, live metrics
```

**Current Workaround:**
```bash
flowctl sandbox start --pipeline examples/pipeline.yaml
flowctl sandbox logs
# Manual, non-integrated experience
```

**Implementation Requirements:**
- File watcher for component changes
- Process orchestrator for local services
- TUI with real-time logs and metrics
- Auto-rebuild for Go/Rust/TypeScript components

### 3. `flowctl add` - Critical Missing Command

**What We Need:**
```bash
flowctl add stellar-source@^1.2.0
# Should: fetch template, pin version, update flow.yml
```

**Current State:** No component marketplace or template system exists.

**Implementation Requirements:**
- Component registry/marketplace
- Template fetching and scaffolding
- Version management and pinning
- Dependency resolution

### 4. `flowctl run` - Good but Needs Enhancement

**Current Implementation:**
```bash
flowctl run config.yaml
# Runs indefinitely until stopped
```

**Proposed Enhancement:**
```bash
flowctl run
# Should: run and exit when DAG completes (for CI/backfills)
# Should: auto-detect flow.yml in current directory
```

**Gap:** No completion detection, requires explicit config file.

### 5. `flowctl apply` - Needs Diff & Deploy Features

**Current Implementation:**
```bash
flowctl apply config.yaml
# Basic deployment
```

**Proposed Enhancement:**
```bash
flowctl apply
# Should: show diff, confirm changes, deploy to target runtime
# Should: support multiple backends (Nomad first, k8s later)
```

**Gap:** No diff preview, no runtime abstraction layer.

### 6. `flowctl logs` vs `flowctl status`

**Current `flowctl logs`:**
```bash
flowctl logs pipeline/my-pipeline
# Basic log viewing
```

**Current `flowctl list`:**
```bash
flowctl list pipelines
# Lists resources
```

**Proposed `flowctl status`:**
```bash
flowctl status
# Should: show health, metrics, lag, component status
```

**Gap:** Need unified status view with health checks and metrics.

## Architectural Implications

### 1. **Single Binary Supervisor** ✅ Partially Aligned
- Current: Has server mode, but not embedded
- Needed: Embed orchestrator for `dev`/`run`, talk to Nomad API for `apply`

### 2. **Component Spec Flexibility** ❌ Currently Missing
- Current: Only supports container-based components
- Needed: Support OCI images OR local binaries for gradual migration

### 3. **Edge-Driven DAG** ✅ Well Aligned
- Current: Has DAG pipeline support with YAML configuration
- Needed: Make queue/pipe configuration explicit

### 4. **Event Bus for Observability** ❌ Currently Missing
- Current: Basic logging
- Needed: Structured events for logs/metrics/alerts

## Migration Strategy Recommendations

### **Phase 1: Enhance Existing Commands (Week 1-2)**
1. **Enhance `flowctl init`**:
   - Add working example pipeline
   - Include CI templates and README
   - Switch from workspace to project model

2. **Improve `flowctl run`**:
   - Add completion detection
   - Auto-detect flow.yml
   - Better error messages with component context

3. **Upgrade `flowctl logs`**:
   - Add component-specific logging
   - Support `flowctl logs component-name` syntax
   - Add follow mode improvements

### **Phase 2: Add Critical Missing Commands (Week 3-4)**
1. **Implement `flowctl dev`**:
   - Merge sandbox functionality
   - Add file watching and hot-reload
   - Create TUI with live logs and metrics
   - Embed process orchestrator

2. **Create `flowctl add`**:
   - Build component template system
   - Implement basic marketplace/registry
   - Add version pinning and dependency management

### **Phase 3: Polish and Production Features (Week 5-6)**
1. **Enhance `flowctl apply`**:
   - Add diff preview
   - Implement Nomad backend
   - Add deployment versioning for rollback

2. **Add Missing Commands**:
   - `flowctl completion`
   - `flowctl rollback`
   - `flowctl package` (stretch goal)

### **Phase 4: Deprecate/Refactor Anti-Patterns**
1. **Hide `flowctl server`**: Embed in dev/apply modes
2. **Remove `flowctl translate`**: Hide behind apply
3. **Deprecate `flowctl sandbox`**: Functionality moved to `flowctl dev`

## Risk Assessment

### **High Risk - Major Architectural Changes**
- **Component spec flexibility**: Requires rethinking how components are defined and executed
- **Single binary supervisor**: May require significant refactoring of current server architecture
- **Event bus**: New observability infrastructure needed

### **Medium Risk - Feature Gaps**
- **`flowctl dev` implementation**: Complex TUI and process management
- **Component marketplace**: New registry/template system
- **Deployment backends**: Nomad/Kubernetes abstraction layer

### **Low Risk - Enhancements**
- **Command improvements**: Mostly additive changes
- **CLI polish**: Standard Cobra features
- **Configuration changes**: YAML structure updates

## Conclusion

The current flowctl has a **solid foundation** but needs **significant DX enhancements** to achieve the proposed vision. The biggest gaps are:

1. **Missing `flowctl dev`** - This is the core developer experience command
2. **Missing `flowctl add`** - Component discovery and reuse
3. **Architectural complexity** - Server/translate commands expose too much complexity

**Recommendation**: Focus on Phase 1-2 enhancements first, as they provide the highest DX impact with manageable risk. The existing command structure provides a good foundation, but the developer experience needs substantial improvement to achieve "minutes-to-first-event" goal.