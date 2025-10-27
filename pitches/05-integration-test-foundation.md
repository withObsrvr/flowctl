# Pitch: Build Integration Test Foundation

## Problem
**Test coverage is almost entirely unit tests**. No integration tests that actually:
- Start real containers
- Run complete pipelines end-to-end
- Verify data flows through the system
- Test Docker Compose generation + execution
- Test control plane registration

**Current state**: 10 test files, all unit tests. No confidence that pieces work together.

**Risk**: Features work in isolation but fail when integrated. Bugs only found manually.

## Appetite
**2 weeks** - This is foundational infrastructure. Take time to do it right.

## Solution (Fat-marker sketch)

### Test Pyramid for flowctl
```
                  /\
                 /  \
                /E2E \        ← Add this layer
               /______\
              /        \
             /Integration\    ← Add this layer
            /____________\
           /              \
          /   Unit Tests   \  ← Already exists
         /__________________\
```

### Three Test Suites to Build

#### 1. Integration Tests (internal/*)
Test component integration without full deployment:
- **Pipeline DAG** → Components → Event flow
- **Translator** → Generator → Valid output
- **Control Plane** → Component registration → Health checks
- **Storage** → Persistence → Retrieval

#### 2. E2E Tests (test/e2e/*)
Test actual deployments:
- **Apply pipeline** → Containers start → Data flows → Get status
- **Delete pipeline** → Containers stop → Cleanup verified
- **Pipeline failure** → Error handling → Recovery

#### 3. Example Validation Tests (test/examples/*)
Every example in `examples/` gets a test:
- Translates without error
- Validates against schema (when CUE is fixed)
- Can be deployed (if Docker available)
- Documented behavior matches actual behavior

### Test Infrastructure
```
test/
├── e2e/
│   ├── framework.go        # Test helpers
│   ├── docker_helper.go    # Container management
│   ├── pipeline_test.go    # E2E pipeline tests
│   └── cli_test.go         # CLI command tests
├── integration/
│   ├── dag_integration_test.go
│   ├── translator_integration_test.go
│   └── controlplane_integration_test.go
└── examples/
    └── examples_test.go    # Validate all examples
```

## Rabbit Holes to Avoid
- ❌ **Don't build a custom test framework from scratch** - Use standard Go testing + testify
- ❌ **Don't add Gherkin/BDD** - Go test functions are fine
- ❌ **Don't test every edge case** - Cover happy path + critical failures
- ❌ **Don't require Kubernetes for tests** - Docker is enough
- ❌ **Don't mock everything** - Real components where possible

## No-Gos (Explicitly Out of Scope)
- Performance/load testing (that's next cycle)
- Chaos engineering tests
- Security scanning
- Fuzz testing
- Browser-based testing
- Test coverage goals (just build the tests, don't get stuck on %)

## Done (Concrete Success Criteria)

### Must Demo This:

**E2E Test: Complete Pipeline Lifecycle**
```bash
go test -v ./test/e2e -run TestPipelineLifecycle

# Output:
# === RUN TestPipelineLifecycle
# --- Starting test environment
# ✓ Docker daemon available
# ✓ Control plane started
# --- Applying pipeline
# ✓ Pipeline translated successfully
# ✓ Containers started
# ✓ Components registered with control plane
# ✓ All components healthy
# --- Testing data flow
# ✓ Source generated events
# ✓ Processor received events
# ✓ Sink wrote events
# --- Cleaning up
# ✓ Pipeline deleted
# ✓ Containers stopped
# ✓ Volumes removed
# --- PASS: TestPipelineLifecycle (15.3s)
```

**Integration Test: Translator**
```bash
go test -v ./test/integration -run TestTranslatorIntegration

# Tests:
# ✓ YAML → Pipeline model parsing
# ✓ Pipeline model → Docker Compose generation
# ✓ Generated compose file is valid
# ✓ All example pipelines translate successfully
```

**Examples Validation**
```bash
go test -v ./test/examples

# Tests each example:
# ✓ minimal.yaml validates
# ✓ dag-pipeline.yaml validates
# ✓ sandbox-pipeline.yaml validates
# --- PASS (all examples valid)
```

### Test Coverage Goals
Not focusing on percentage, but must cover:
- ✓ At least one E2E test per major CLI command
- ✓ At least one integration test per internal package
- ✓ All examples have validation tests
- ✓ Critical error paths tested (network failure, container crash, etc.)

### Code Expected:
- `test/e2e/framework.go` - Test framework (200 LOC)
- `test/e2e/pipeline_test.go` - E2E tests (300+ LOC)
- `test/integration/*_test.go` - Integration tests (500+ LOC)
- `test/examples/examples_test.go` - Example validation (150 LOC)
- `Makefile` - Add `make test-integration`, `make test-e2e` targets
- CI configuration (GitHub Actions, if exists) (50 LOC)

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ E2E test framework with Docker helpers
✓ One E2E test: apply → run → delete pipeline
✓ Integration test for DAG execution
✓ Integration test for translator
✓ All examples validate in tests
✓ Makefile targets for test suites
✓ Tests run in CI (if CI exists)
✓ README section on running tests

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ E2E test for `get` command
○ E2E test for `logs` command
○ Integration test for control plane client
○ Parallel test execution
○ Test fixtures for common pipelines
○ Docker image caching for faster tests

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ E2E tests for all CLI commands
○ Integration tests for every package
○ Performance benchmarks
○ Test coverage reporting
○ Automated test result analysis
○ Test environment teardown verification
```

## Week 1: Integration Tests
**Goal**: Test components working together, no Docker needed

- Day 1-2: Framework setup, DAG integration test
- Day 3: Translator integration test
- Day 4: Control plane integration test
- Day 5: Example validation tests

## Week 2: E2E Tests
**Goal**: Real deployments, containers running

- Day 1-2: E2E framework with Docker helpers
- Day 3: Pipeline lifecycle E2E test
- Day 4: CLI command E2E tests (get, logs if implemented)
- Day 5: CI integration, documentation, polish

## Success Metric
```bash
make test-all

# Runs:
# - Unit tests (existing)
# - Integration tests (new)
# - E2E tests (new)
# - Example validation (new)

# All pass ✓
```

## Hill Progress Indicators

**Left side (figuring it out):**
- Test framework structure decided
- Docker test helpers working
- Can start/stop containers in tests
- One integration test running
- One E2E test running

**Right side (making it happen):**
- Full integration test suite
- Full E2E test suite
- Example validation tests
- CI integration complete
- Documentation written
