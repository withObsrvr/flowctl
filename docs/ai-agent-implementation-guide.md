# AI Agent Integration: Implementation Guide

## Quick Start: Making flowctl AI-Ready Today

This guide provides practical steps to implement AI agent integration with flowctl, starting with minimal changes and building toward the full vision.

## Phase 1: Immediate Implementation (Week 1-2)

### 1. JSON Output Mode for All Commands

Make flowctl output machine-readable by default:

```go
// internal/cmd/flags.go
type OutputFormat string

const (
    OutputHuman OutputFormat = "human"
    OutputJSON  OutputFormat = "json"
    OutputYAML  OutputFormat = "yaml"
)

type GlobalFlags struct {
    Output OutputFormat `json:"output_format"`
    // For AI agents to identify themselves
    Agent  string      `json:"agent,omitempty"`
}
```

### 2. Structured Error Responses

```go
// internal/errors/ai_errors.go
type AIError struct {
    Code        string                 `json:"code"`
    Message     string                 `json:"message"`
    Details     map[string]interface{} `json:"details,omitempty"`
    Suggestions []string               `json:"suggestions,omitempty"`
    Context     map[string]interface{} `json:"context,omitempty"`
}

// Example usage
return &AIError{
    Code:    "PIPELINE_START_FAILED",
    Message: "Failed to start pipeline",
    Details: map[string]interface{}{
        "pipeline_id": pipelineID,
        "reason":      "component 'processor-1' failed to initialize",
    },
    Suggestions: []string{
        "Check if processor-1 binary exists",
        "Verify processor-1 has correct permissions",
        "Review processor-1 logs for detailed error",
    },
}
```

### 3. Command Introspection Endpoint

```go
// cmd/describe.go
var describeCmd = &cobra.Command{
    Use:   "describe [command]",
    Short: "Describe available commands in machine-readable format",
    RunE: func(cmd *cobra.Command, args []string) error {
        description := CommandDescription{
            Name:        cmd.Name(),
            Description: cmd.Short,
            Usage:       cmd.Use,
            Flags:       extractFlags(cmd),
            Examples:    extractExamples(cmd),
            Version:     "1.0",
        }
        
        return outputJSON(description)
    },
}

type CommandDescription struct {
    Name        string                `json:"name"`
    Description string                `json:"description"`
    Usage       string                `json:"usage"`
    Flags       []FlagDescription     `json:"flags"`
    Examples    []Example             `json:"examples"`
    Version     string                `json:"api_version"`
}
```

## Phase 2: Tool Calling Interface (Week 3-4)

### 1. OpenAI-Compatible Function Definitions

```go
// internal/ai/tools/registry.go
package tools

type Tool struct {
    Type     string     `json:"type"`
    Function Function   `json:"function"`
}

type Function struct {
    Name        string     `json:"name"`
    Description string     `json:"description"`
    Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
    Type       string              `json:"type"`
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required"`
}

func GetToolDefinitions() []Tool {
    return []Tool{
        {
            Type: "function",
            Function: Function{
                Name:        "create_pipeline",
                Description: "Create a new data processing pipeline",
                Parameters: Parameters{
                    Type: "object",
                    Properties: map[string]Property{
                        "name": {
                            Type:        "string",
                            Description: "Name of the pipeline",
                        },
                        "source": {
                            Type:        "object",
                            Description: "Source configuration",
                        },
                        "processors": {
                            Type:        "array",
                            Description: "List of processors",
                        },
                    },
                    Required: []string{"name", "source"},
                },
            },
        },
    }
}
```

### 2. AI Gateway Command

```go
// cmd/ai/gateway.go
var aiGatewayCmd = &cobra.Command{
    Use:   "ai-gateway",
    Short: "AI agent gateway for tool execution",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Read tool call from stdin (or HTTP in server mode)
        var toolCall ToolCall
        if err := json.NewDecoder(os.Stdin).Decode(&toolCall); err != nil {
            return err
        }
        
        // Execute tool
        result, err := executeToolCall(toolCall)
        if err != nil {
            return outputAIError(err)
        }
        
        // Return result
        return outputJSON(ToolResult{
            ToolCallID: toolCall.ID,
            Result:     result,
        })
    },
}
```

## Phase 3: AI Components (Week 5-6)

### 1. AI Processor Interface

```go
// internal/processor/ai_processor.go
type AIProcessor struct {
    BaseProcessor
    client    AIClient
    modelName string
    prompt    string
}

func (p *AIProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
    // Prepare prompt with event data
    fullPrompt := p.preparePrompt(event)
    
    // Call AI model
    response, err := p.client.Complete(ctx, CompleteRequest{
        Model:  p.modelName,
        Prompt: fullPrompt,
        Format: "json",
    })
    if err != nil {
        return nil, fmt.Errorf("AI processing failed: %w", err)
    }
    
    // Parse and validate response
    enrichedData, err := p.parseResponse(response)
    if err != nil {
        return nil, fmt.Errorf("failed to parse AI response: %w", err)
    }
    
    // Create enriched event
    return &Event{
        ID:        event.ID,
        Timestamp: event.Timestamp,
        Data:      enrichedData,
        Metadata:  event.Metadata,
    }, nil
}
```

### 2. Pipeline Template for AI Processing

```yaml
# templates/ai-enrichment-template.yaml
apiVersion: v1
kind: PipelineTemplate
metadata:
  name: ai-enrichment
  annotations:
    ai.obsrvr.com/compatible: "true"
spec:
  parameters:
    - name: model
      description: "AI model to use"
      type: string
      default: "gpt-3.5-turbo"
    - name: prompt
      description: "Prompt template"
      type: string
  
  template: |
    apiVersion: v1
    kind: Pipeline
    metadata:
      name: {{ .Values.name }}
    spec:
      processors:
        - id: ai-enricher
          type: ai-processor
          config:
            model: {{ .Values.model }}
            prompt: {{ .Values.prompt }}
            timeout: 30s
            retry:
              attempts: 3
              backoff: exponential
```

## Phase 4: Natural Language Interface (Week 7-8)

### 1. NL2Pipeline Converter

```go
// internal/ai/nl2pipeline/converter.go
type NL2PipelineConverter struct {
    llm          LLMClient
    componentDB  ComponentDatabase
    templateDB   TemplateDatabase
}

func (c *NL2PipelineConverter) Convert(naturalLanguage string) (*Pipeline, error) {
    // Step 1: Analyze intent
    intent, err := c.analyzeIntent(naturalLanguage)
    if err != nil {
        return nil, err
    }
    
    // Step 2: Extract requirements
    requirements, err := c.extractRequirements(intent)
    if err != nil {
        return nil, err
    }
    
    // Step 3: Find matching components
    components, err := c.findComponents(requirements)
    if err != nil {
        return nil, err
    }
    
    // Step 4: Generate pipeline
    pipeline, err := c.generatePipeline(requirements, components)
    if err != nil {
        return nil, err
    }
    
    // Step 5: Validate
    if err := c.validatePipeline(pipeline); err != nil {
        return nil, err
    }
    
    return pipeline, nil
}
```

### 2. Interactive Pipeline Builder

```go
// cmd/ai/build.go
var aiBuildCmd = &cobra.Command{
    Use:   "build",
    Short: "Build a pipeline using natural language",
    RunE: func(cmd *cobra.Command, args []string) error {
        description := strings.Join(args, " ")
        
        fmt.Println("🤖 Analyzing your requirements...")
        
        converter := nl2pipeline.New()
        pipeline, err := converter.Convert(description)
        if err != nil {
            return err
        }
        
        // Show preview
        fmt.Println("\n📋 Generated Pipeline:")
        fmt.Println(pipeline.ToYAML())
        
        // Interactive refinement
        if interactive {
            pipeline, err = refineInteractively(pipeline)
            if err != nil {
                return err
            }
        }
        
        // Save or apply
        if confirm("Apply this pipeline?") {
            return applyPipeline(pipeline)
        }
        
        return savePipeline(pipeline)
    },
}
```

## Phase 5: Autonomous Operations (Week 9-12)

### 1. Self-Healing Controller

```go
// internal/controller/self_healing.go
type SelfHealingController struct {
    monitor   PipelineMonitor
    analyzer  AIAnalyzer
    executor  ActionExecutor
    policies  []HealingPolicy
}

func (c *SelfHealingController) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        
        case issue := <-c.monitor.Issues():
            go c.handleIssue(ctx, issue)
        }
    }
}

func (c *SelfHealingController) handleIssue(ctx context.Context, issue Issue) {
    // Analyze with AI
    analysis, err := c.analyzer.Analyze(ctx, issue)
    if err != nil {
        log.Error("Failed to analyze issue", "error", err)
        return
    }
    
    // Find applicable policies
    policies := c.findApplicablePolicies(issue, analysis)
    
    // Generate healing plan
    plan, err := c.generateHealingPlan(issue, analysis, policies)
    if err != nil {
        log.Error("Failed to generate healing plan", "error", err)
        return
    }
    
    // Execute with approval if needed
    if plan.RequiresApproval {
        if !c.requestApproval(plan) {
            return
        }
    }
    
    // Execute healing actions
    if err := c.executor.Execute(ctx, plan); err != nil {
        log.Error("Failed to execute healing plan", "error", err)
        return
    }
    
    // Learn from outcome
    c.learnFromOutcome(issue, plan, err)
}
```

### 2. Cost Optimization Agent

```go
// internal/agents/cost_optimizer.go
type CostOptimizer struct {
    metrics   MetricsClient
    pricing   PricingService
    analyzer  AIAnalyzer
    simulator Simulator
}

func (o *CostOptimizer) OptimizePipeline(pipelineID string) (*OptimizationPlan, error) {
    // Get current costs
    costs, err := o.getCurrentCosts(pipelineID)
    if err != nil {
        return nil, err
    }
    
    // Analyze with AI
    prompt := fmt.Sprintf(`
        Analyze this pipeline's cost structure and suggest optimizations:
        
        Current Costs:
        %s
        
        Constraints:
        - Maintain 99.9% SLA
        - No data loss
        - Minimize latency impact
        
        Suggest specific configuration changes to reduce costs.
    `, costs.ToJSON())
    
    suggestions, err := o.analyzer.Analyze(prompt)
    if err != nil {
        return nil, err
    }
    
    // Simulate each suggestion
    validSuggestions := []Suggestion{}
    for _, suggestion := range suggestions {
        result, err := o.simulator.Simulate(pipelineID, suggestion)
        if err != nil {
            continue
        }
        
        if result.MeetsSLA && result.CostReduction > 0 {
            validSuggestions = append(validSuggestions, suggestion)
        }
    }
    
    return &OptimizationPlan{
        PipelineID:        pipelineID,
        CurrentCost:       costs.Total,
        ProjectedSavings:  calculateSavings(validSuggestions),
        Suggestions:       validSuggestions,
        ImplementationPlan: generateImplementationPlan(validSuggestions),
    }, nil
}
```

## Testing AI Integration

### 1. Mock AI Agent Test

```python
# test/ai_agent_test.py
import json
import subprocess

def test_pipeline_creation():
    # Simulate AI agent tool call
    tool_call = {
        "id": "call_123",
        "type": "function",
        "function": {
            "name": "create_pipeline",
            "arguments": json.dumps({
                "name": "test-pipeline",
                "source": {
                    "type": "kafka",
                    "topic": "test-events"
                },
                "processors": [{
                    "type": "filter",
                    "condition": "value > 100"
                }],
                "sink": {
                    "type": "stdout"
                }
            })
        }
    }
    
    # Execute via AI gateway
    result = subprocess.run(
        ["flowctl", "ai-gateway"],
        input=json.dumps(tool_call),
        capture_output=True,
        text=True
    )
    
    assert result.returncode == 0
    response = json.loads(result.stdout)
    assert response["tool_call_id"] == "call_123"
    assert "pipeline_id" in response["result"]
```

### 2. Natural Language Test

```bash
#!/bin/bash
# test/nl_pipeline_test.sh

# Test natural language pipeline creation
OUTPUT=$(flowctl ai build "Create a pipeline that reads from Kafka topic 'orders', 
         filters orders over $1000, enriches with customer data, 
         and sends alerts for VIP customers")

# Check if pipeline was generated
echo "$OUTPUT" | grep -q "Generated Pipeline"
assert_eq $? 0 "Pipeline generation failed"

# Verify pipeline components
echo "$OUTPUT" | grep -q "kafka-source"
assert_eq $? 0 "Kafka source not found"

echo "$OUTPUT" | grep -q "filter"
assert_eq $? 0 "Filter processor not found"

echo "$OUTPUT" | grep -q "enricher"
assert_eq $? 0 "Enricher processor not found"
```

## Security Considerations

### 1. AI Action Validation

```go
// internal/security/ai_validator.go
type AIActionValidator struct {
    policies []SecurityPolicy
    auditor  Auditor
}

func (v *AIActionValidator) Validate(action AIAction) error {
    // Check against security policies
    for _, policy := range v.policies {
        if err := policy.Validate(action); err != nil {
            v.auditor.LogRejection(action, err)
            return fmt.Errorf("security policy violation: %w", err)
        }
    }
    
    // Check resource limits
    if err := v.checkResourceLimits(action); err != nil {
        return err
    }
    
    // Audit approved action
    v.auditor.LogApproval(action)
    
    return nil
}
```

### 2. Rate Limiting for AI Agents

```go
// internal/middleware/ai_rate_limit.go
func AIRateLimitMiddleware(limiter RateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            agentID := r.Header.Get("X-AI-Agent-ID")
            if agentID == "" {
                agentID = "anonymous"
            }
            
            if !limiter.Allow(agentID) {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusTooManyRequests)
                json.NewEncoder(w).Encode(AIError{
                    Code:    "RATE_LIMIT_EXCEEDED",
                    Message: "Too many requests from AI agent",
                    Details: map[string]interface{}{
                        "retry_after": limiter.RetryAfter(agentID),
                    },
                })
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

## Deployment Guide

### 1. Enable AI Features

```yaml
# config/flowctl.yaml
ai:
  enabled: true
  
  # AI provider configuration
  providers:
    openai:
      api_key: ${OPENAI_API_KEY}
      model: "gpt-4"
    
    anthropic:
      api_key: ${ANTHROPIC_API_KEY}
      model: "claude-3-opus"
  
  # Feature flags
  features:
    natural_language: true
    self_healing: true
    cost_optimization: true
    ai_components: true
  
  # Security
  security:
    require_approval: true
    audit_all_actions: true
    rate_limits:
      requests_per_minute: 60
      tokens_per_hour: 100000
```

### 2. Start AI-Enabled flowctl

```bash
# Start with AI features
flowctl run my-pipeline.yaml --ai-enabled

# Start AI gateway server
flowctl ai-gateway serve --port 8081

# Enable autonomous operations
flowctl ai autonomous --pipeline my-pipeline --policies policies.yaml
```

## Monitoring AI Integration

### 1. AI-Specific Metrics

```yaml
# monitoring/ai-metrics.yaml
metrics:
  - name: ai_requests_total
    type: counter
    description: Total AI requests by agent and operation
    labels: [agent_id, operation, model]
  
  - name: ai_request_duration_seconds
    type: histogram
    description: AI request processing time
    labels: [operation, model]
  
  - name: ai_token_usage_total
    type: counter
    description: Total AI tokens consumed
    labels: [agent_id, model, operation]
  
  - name: ai_actions_approved_total
    type: counter
    description: AI actions approved vs rejected
    labels: [action_type, status]
  
  - name: ai_cost_dollars
    type: gauge
    description: AI usage cost in dollars
    labels: [model, operation]
```

### 2. AI Dashboard

```go
// internal/dashboard/ai_dashboard.go
func RegisterAIDashboard(mux *http.ServeMux) {
    mux.HandleFunc("/ai/dashboard", func(w http.ResponseWriter, r *http.Request) {
        metrics := collectAIMetrics()
        
        dashboard := Dashboard{
            Title: "AI Integration Status",
            Panels: []Panel{
                {
                    Title: "AI Requests",
                    Type:  "graph",
                    Data:  metrics.RequestsOverTime,
                },
                {
                    Title: "Token Usage",
                    Type:  "gauge",
                    Data:  metrics.TokenUsage,
                },
                {
                    Title: "Cost Tracking",
                    Type:  "number",
                    Data:  metrics.TotalCost,
                },
                {
                    Title: "Agent Activity",
                    Type:  "table",
                    Data:  metrics.AgentActivity,
                },
            },
        }
        
        renderDashboard(w, dashboard)
    })
}
```

## Next Steps

1. **Start Small**: Implement JSON output and structured errors first
2. **Add Tool Interface**: Build the AI gateway for tool calling
3. **Test with Real Agents**: Use GPT-4 or Claude to test the interfaces
4. **Iterate Based on Feedback**: Refine based on real-world usage
5. **Scale Gradually**: Add more sophisticated features as needed

This practical guide provides a roadmap for implementing AI agent integration with flowctl, starting with simple changes and building toward autonomous pipeline management.