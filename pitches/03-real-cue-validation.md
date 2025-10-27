# Pitch: Fix CUE Schema Validation (Currently Disabled)

## Problem
The CUE validator (`internal/translator/cue_validator.go`) has **140 lines of commented-out code**. The actual `Validate()` method on line 65 just calls the basic validator instead:

```go
func (v *CUEValidator) Validate(pipeline *model.Pipeline) error {
    // TODO: Fix CUE schema loading - for now, use basic validator
    return basicValidator.Validate(pipeline)
}
```

**Impact**:
- Users lose the benefit of strong schema validation
- Invalid pipelines accepted at translate time, fail at runtime
- Promised feature in docs doesn't work
- Type safety compromised

## Appetite
**1 week** - Clear problem, known solution (code is already written, just broken).

## Solution (Fat-marker sketch)

### Two-Phase Approach

**Phase 1: Make existing CUE code work** (3 days)
1. Uncomment the 140 lines
2. Fix the schema loading issue (probably a path problem)
3. Debug why CUE instance creation fails
4. Get the test passing

**Phase 2: Integration** (2 days)
1. Hook up CUE validator in translation path
2. Add helpful error messages when validation fails
3. Test with all example pipelines
4. Document schema extensions

### Root Cause Analysis
Looking at the commented code:
- Lines 68-140: Full CUE implementation exists
- Likely issue: Schema file path incorrect or schema syntax error
- Simple fix: Debug the schema loading

### Quick Win Path
```go
// Current (broken):
schemaBytes, err := os.ReadFile("schemas/cue/schema.cue")
// Probably fails because of relative path issues

// Fixed:
schemaPath := filepath.Join(getRootDir(), "schemas/cue/schema.cue")
schemaBytes, err := os.ReadFile(schemaPath)
```

## Rabbit Holes to Avoid
- ❌ **Don't rewrite the schema from scratch** - It exists and is probably fine
- ❌ **Don't add CUE code generation** - Just validation for now
- ❌ **Don't build a schema migration system** - Keep it simple
- ❌ **Don't add schema versioning yet** - One schema, version it later
- ❌ **Don't integrate with external schema registries** - Local file is enough

## No-Gos (Explicitly Out of Scope)
- CUE-based code generation for Go types
- Schema evolution/migration tools
- Multiple schema versions support
- Schema documentation auto-generation
- Custom CUE validation plugins
- IDE integration for schema validation

## Done (Concrete Success Criteria)

### Must Demo This:

**Scenario 1: Valid Pipeline Passes**
```bash
# Valid pipeline
./bin/flowctl translate examples/minimal.yaml

# ✓ CUE validation runs
# ✓ Validation passes
# ✓ Generates docker-compose.yml
```

**Scenario 2: Invalid Pipeline Fails with Clear Error**
```bash
# Create invalid pipeline (missing required field)
cat > bad.yaml <<EOF
apiVersion: v1alpha1
kind: Pipeline
metadata:
  name: broken
spec:
  sources:
    - id: test
      # Missing required 'image' field
      command: ["./run"]
EOF

./bin/flowctl translate bad.yaml

# ✓ CUE validation runs
# ✗ Fails with clear message:
#   "Validation error: sources[0].image is required"
# ✓ Points to line number in YAML
# ✓ Suggests fix
```

**Scenario 3: Type Mismatch Caught**
```bash
# Invalid type (string instead of int)
cat > bad2.yaml <<EOF
spec:
  sources:
    - id: test
      image: foo
      ports:
        - containerPort: "not a number"
EOF

./bin/flowctl translate bad2.yaml

# ✓ Fails: "containerPort must be an integer, got string"
```

### Code Changes Expected:
- `internal/translator/cue_validator.go` - Uncomment and fix (140 LOC already written!)
- `internal/translator/cue_validator_test.go` - Add more test cases (100 LOC)
- `schemas/cue/schema.cue` - Potential fixes to schema (20-50 LOC)
- Error message formatter for CUE errors (50 LOC)

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ CUE validation actually runs (uncomment code)
✓ Schema file loads correctly from disk
✓ Valid pipelines pass validation
✓ Invalid pipelines fail with clear error messages
✓ All existing example pipelines validate successfully
✓ Test coverage for validation cases

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ Line numbers in error messages
○ Multiple errors reported (not just first one)
○ Suggestions for common mistakes
○ Schema reference documentation
○ Validate command (separate from translate)

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ JSON Schema export for IDE autocomplete
○ Schema linting
○ Custom error messages per field
○ Validation warnings (non-fatal issues)
○ Schema unit tests (validate the schema itself)
```

## Why This Is Quick
The hard work is already done! The code exists in commented form. This is primarily a debugging exercise:

1. **Day 1**: Uncomment code, find why schema loading fails
2. **Day 2**: Fix schema loading, get basic validation working
3. **Day 3**: Test with all examples, fix any schema issues
4. **Day 4**: Improve error messages
5. **Day 5**: Documentation, final polish

## Risk Mitigation
If CUE integration proves more complex than expected:
- **Escape hatch**: Keep basic validator as fallback
- **Hybrid approach**: CUE for schema validation, basic for runtime checks
- Still ship improvement over current state (which is broken)

## Hill Progress Indicators

**Left side (figuring it out):**
- Understand why original implementation was commented out
- Schema file loads successfully
- CUE instance creates without error
- One example validates successfully

**Right side (making it happen):**
- All examples validate
- Error messages are clear and helpful
- Tests written and passing
- Documentation updated
- Basic validator removed
