package generator

import (
	"fmt"
	"strings"
	
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// KubernetesGenerator generates Kubernetes manifests and implements interfaces.Generator
type KubernetesGenerator struct{}

// NewKubernetesGenerator creates a new Kubernetes generator
func NewKubernetesGenerator() *KubernetesGenerator {
	return &KubernetesGenerator{}
}

// Generate produces Kubernetes manifests
func (g *KubernetesGenerator) Generate(pipeline *model.Pipeline, opts model.TranslationOptions) ([]byte, error) {
	logger.Debug("Generating Kubernetes manifests")

	// Validate the pipeline for Kubernetes compatibility
	if err := g.Validate(pipeline); err != nil {
		return nil, err
	}

	// Create a list to hold all Kubernetes resources
	resources := []interface{}{}

	// Add namespace if specified
	if pipeline.Metadata.Namespace != "" {
		namespace := g.generateNamespace(pipeline)
		resources = append(resources, namespace)
	}

	// Add ConfigMap for pipeline configuration
	configMap := g.generateConfigMap(pipeline, opts)
	resources = append(resources, configMap)

	// Generate resources for sources
	for _, src := range pipeline.Spec.Sources {
		deployment := g.generateDeployment(src, "source", pipeline, opts)
		resources = append(resources, deployment)

		// Generate service if the component exposes ports
		if len(src.Ports) > 0 {
			service := g.generateService(src, pipeline, opts)
			resources = append(resources, service)
		}
	}

	// Generate resources for processors
	for _, proc := range pipeline.Spec.Processors {
		deployment := g.generateDeployment(proc, "processor", pipeline, opts)
		resources = append(resources, deployment)

		// Generate service if the component exposes ports
		if len(proc.Ports) > 0 {
			service := g.generateService(proc, pipeline, opts)
			resources = append(resources, service)
		}
	}

	// Generate resources for sinks
	for _, sink := range pipeline.Spec.Sinks {
		deployment := g.generateDeployment(sink, "sink", pipeline, opts)
		resources = append(resources, deployment)

		// Generate service if the component exposes ports
		if len(sink.Ports) > 0 {
			service := g.generateService(sink, pipeline, opts)
			resources = append(resources, service)
		}
	}

	// Convert resources to YAML
	var result strings.Builder
	for i, resource := range resources {
		data, err := yaml.Marshal(resource)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal resource: %w", err)
		}
		result.Write(data)
		
		// Add separator between resources
		if i < len(resources)-1 {
			result.WriteString("---\n")
		}
	}

	return []byte(result.String()), nil
}

// Validate checks if the pipeline can be translated to Kubernetes
func (g *KubernetesGenerator) Validate(pipeline *model.Pipeline) error {
	logger.Debug("Validating pipeline for Kubernetes compatibility")

	// Check required fields
	if pipeline.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}

	if pipeline.Kind != "Pipeline" {
		return fmt.Errorf("kind must be 'Pipeline'")
	}

	if pipeline.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}

	if len(pipeline.Spec.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}

	// Check that all components have required fields
	for _, src := range pipeline.Spec.Sources {
		if src.ID == "" {
			return fmt.Errorf("source id is required")
		}
		if src.Image == "" {
			return fmt.Errorf("source image is required for %s", src.ID)
		}
	}

	for _, proc := range pipeline.Spec.Processors {
		if proc.ID == "" {
			return fmt.Errorf("processor id is required")
		}
		if proc.Image == "" {
			return fmt.Errorf("processor image is required for %s", proc.ID)
		}
	}

	for _, sink := range pipeline.Spec.Sinks {
		if sink.ID == "" {
			return fmt.Errorf("sink id is required")
		}
		if sink.Image == "" {
			return fmt.Errorf("sink image is required for %s", sink.ID)
		}
	}

	return nil
}

// Helper functions

// generateNamespace creates a Kubernetes Namespace
func (g *KubernetesGenerator) generateNamespace(pipeline *model.Pipeline) map[string]interface{} {
	namespace := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": pipeline.Metadata.Namespace,
		},
	}

	// Add labels if present
	if len(pipeline.Metadata.Labels) > 0 {
		namespace["metadata"].(map[string]interface{})["labels"] = pipeline.Metadata.Labels
	}

	return namespace
}

// generateConfigMap creates a ConfigMap for pipeline configuration
func (g *KubernetesGenerator) generateConfigMap(pipeline *model.Pipeline, opts model.TranslationOptions) map[string]interface{} {
	// Create a prefix for resources if provided
	prefix := ""
	if opts.ResourcePrefix != "" {
		prefix = opts.ResourcePrefix + "-"
	}

	configMapName := fmt.Sprintf("%s%s-config", prefix, pipeline.Metadata.Name)
	
	// Serialize pipeline config to YAML for the ConfigMap
	pipelineYAML, err := yaml.Marshal(pipeline)
	if err != nil {
		logger.Error("Failed to marshal pipeline to YAML", zap.Error(err))
		pipelineYAML = []byte("{}")
	}

	configMap := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      configMapName,
			"namespace": pipeline.Metadata.Namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/name":     pipeline.Metadata.Name,
				"app.kubernetes.io/part-of":  "obsrvr",
				"app.kubernetes.io/managed-by": "flowctl",
			},
		},
		"data": map[string]interface{}{
			"pipeline.yaml": string(pipelineYAML),
		},
	}

	// Add custom labels if present
	if len(pipeline.Metadata.Labels) > 0 {
		labels := configMap["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		for k, v := range pipeline.Metadata.Labels {
			labels[k] = v
		}
	}

	// Add custom annotations if present
	if len(pipeline.Metadata.Annotations) > 0 {
		configMap["metadata"].(map[string]interface{})["annotations"] = pipeline.Metadata.Annotations
	}

	return configMap
}

// generateDeployment creates a Kubernetes Deployment for a component
func (g *KubernetesGenerator) generateDeployment(component model.Component, 
	componentType string, pipeline *model.Pipeline, opts model.TranslationOptions) map[string]interface{} {
	
	// Create a prefix for resources if provided
	prefix := ""
	if opts.ResourcePrefix != "" {
		prefix = opts.ResourcePrefix + "-"
	}

	deploymentName := fmt.Sprintf("%s%s", prefix, sanitizeResourceName(component.ID))
	
	// Determine the number of replicas
	replicas := 1
	if component.Replicas > 0 {
		replicas = component.Replicas
	}

	// Build container definition
	container := map[string]interface{}{
		"name":            sanitizeResourceName(component.ID),
		"image":           getKubernetesImageRef(component.Image, opts.RegistryPrefix),
		"imagePullPolicy": "IfNotPresent",
	}

	// Add command if specified
	if len(component.Command) > 0 {
		container["command"] = component.Command
	}

	// Add environment variables
	envVars := mergeKubernetesEnvs(component.Env, opts.EnvVars)
	if len(envVars) > 0 {
		env := []map[string]interface{}{}
		for name, value := range envVars {
			env = append(env, map[string]interface{}{
				"name":  name,
				"value": value,
			})
		}
		container["env"] = env
	}

	// Add ports
	if len(component.Ports) > 0 {
		ports := []map[string]interface{}{}
		for _, p := range component.Ports {
			portName := p.Name
			if portName == "" {
				portName = fmt.Sprintf("port-%d", p.ContainerPort)
			}
			
			port := map[string]interface{}{
				"name":          portName,
				"containerPort": p.ContainerPort,
				"protocol":      "TCP",
			}
			
			// Set protocol if specified
			if p.Protocol != "" {
				port["protocol"] = p.Protocol
			}
			
			ports = append(ports, port)
		}
		container["ports"] = ports
	}

	// Add volume mounts
	if len(component.Volumes) > 0 {
		volumeMounts := []map[string]interface{}{}
		for _, v := range component.Volumes {
			volumeMount := map[string]interface{}{
				"name":      sanitizeResourceName(v.Name),
				"mountPath": v.MountPath,
			}
			volumeMounts = append(volumeMounts, volumeMount)
		}
		container["volumeMounts"] = volumeMounts
	}

	// Add health check probe if specified
	if component.HealthCheck != "" {
		port := component.HealthPort
		if port == 0 && len(component.Ports) > 0 {
			port = component.Ports[0].ContainerPort
		}
		
		// Use HTTP probe by default
		httpPath := component.HealthCheck
		// If it's a full URL, extract just the path
		if strings.HasPrefix(httpPath, "http://") {
			parts := strings.SplitN(httpPath, "/", 4)
			if len(parts) >= 4 {
				httpPath = "/" + parts[3]
			} else {
				httpPath = "/"
			}
		}
		
		// Create HTTP probe
		probe := map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": httpPath,
				"port": port,
			},
			"initialDelaySeconds": 10,
			"periodSeconds":       30,
			"timeoutSeconds":      5,
			"failureThreshold":    3,
		}
		
		container["livenessProbe"] = probe
		container["readinessProbe"] = probe
	}

	// Add resource requirements if specified
	if component.Resources.Requests.CPU != "" || component.Resources.Requests.Memory != "" ||
		component.Resources.Limits.CPU != "" || component.Resources.Limits.Memory != "" {
		
		resources := map[string]interface{}{}
		
		if component.Resources.Requests.CPU != "" || component.Resources.Requests.Memory != "" {
			requests := map[string]interface{}{}
			if component.Resources.Requests.CPU != "" {
				requests["cpu"] = component.Resources.Requests.CPU
			}
			if component.Resources.Requests.Memory != "" {
				requests["memory"] = component.Resources.Requests.Memory
			}
			resources["requests"] = requests
		}
		
		if component.Resources.Limits.CPU != "" || component.Resources.Limits.Memory != "" {
			limits := map[string]interface{}{}
			if component.Resources.Limits.CPU != "" {
				limits["cpu"] = component.Resources.Limits.CPU
			}
			if component.Resources.Limits.Memory != "" {
				limits["memory"] = component.Resources.Limits.Memory
			}
			resources["limits"] = limits
		}
		
		container["resources"] = resources
	}

	// Create deployment
	deployment := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      deploymentName,
			"namespace": pipeline.Metadata.Namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/name":       pipeline.Metadata.Name,
				"app.kubernetes.io/component":  componentType,
				"app.kubernetes.io/instance":   component.ID,
				"app.kubernetes.io/part-of":    "obsrvr",
				"app.kubernetes.io/managed-by": "flowctl",
			},
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app.kubernetes.io/name":      pipeline.Metadata.Name,
					"app.kubernetes.io/component": componentType,
					"app.kubernetes.io/instance":  component.ID,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/name":      pipeline.Metadata.Name,
						"app.kubernetes.io/component": componentType,
						"app.kubernetes.io/instance":  component.ID,
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{container},
				},
			},
		},
	}

	// Add volumes if needed
	if len(component.Volumes) > 0 {
		volumes := []map[string]interface{}{}
		for _, v := range component.Volumes {
			volume := map[string]interface{}{
				"name": sanitizeResourceName(v.Name),
			}
			
			// If hostPath is specified, use hostPath volume
			if v.HostPath != "" {
				volume["hostPath"] = map[string]interface{}{
					"path": v.HostPath,
				}
			} else {
				// Otherwise create an emptyDir volume
				volume["emptyDir"] = map[string]interface{}{}
			}
			
			volumes = append(volumes, volume)
		}
		
		podSpec := deployment["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
		podSpec["volumes"] = volumes
	}

	// Add custom labels if present
	if len(pipeline.Metadata.Labels) > 0 {
		deploymentLabels := deployment["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		podLabels := deployment["spec"].(map[string]interface{})["template"].(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		
		for k, v := range pipeline.Metadata.Labels {
			deploymentLabels[k] = v
			podLabels[k] = v
		}
	}

	// Add custom annotations if present
	if len(pipeline.Metadata.Annotations) > 0 {
		deployment["metadata"].(map[string]interface{})["annotations"] = pipeline.Metadata.Annotations
		
		// Also add annotations to pod template
		podMeta := deployment["spec"].(map[string]interface{})["template"].(map[string]interface{})["metadata"].(map[string]interface{})
		podMeta["annotations"] = pipeline.Metadata.Annotations
	}

	return deployment
}

// generateService creates a Kubernetes Service for a component
func (g *KubernetesGenerator) generateService(component model.Component, 
	pipeline *model.Pipeline, opts model.TranslationOptions) map[string]interface{} {
	
	// Create a prefix for resources if provided
	prefix := ""
	if opts.ResourcePrefix != "" {
		prefix = opts.ResourcePrefix + "-"
	}

	serviceName := fmt.Sprintf("%s%s", prefix, sanitizeResourceName(component.ID))
	
	// Determine component type
	componentType := "unknown"
	for _, src := range pipeline.Spec.Sources {
		if src.ID == component.ID {
			componentType = "source"
			break
		}
	}
	if componentType == "unknown" {
		for _, proc := range pipeline.Spec.Processors {
			if proc.ID == component.ID {
				componentType = "processor"
				break
			}
		}
	}
	if componentType == "unknown" {
		for _, sink := range pipeline.Spec.Sinks {
			if sink.ID == component.ID {
				componentType = "sink"
				break
			}
		}
	}

	// Create ports for service
	ports := []map[string]interface{}{}
	for _, p := range component.Ports {
		portName := p.Name
		if portName == "" {
			portName = fmt.Sprintf("port-%d", p.ContainerPort)
		}
		
		port := map[string]interface{}{
			"name":       portName,
			"port":       p.ContainerPort,
			"targetPort": p.ContainerPort,
			"protocol":   "TCP",
		}
		
		// Set protocol if specified
		if p.Protocol != "" {
			port["protocol"] = p.Protocol
		}
		
		ports = append(ports, port)
	}

	// Create service
	service := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      serviceName,
			"namespace": pipeline.Metadata.Namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/name":       pipeline.Metadata.Name,
				"app.kubernetes.io/component":  componentType,
				"app.kubernetes.io/instance":   component.ID,
				"app.kubernetes.io/part-of":    "obsrvr",
				"app.kubernetes.io/managed-by": "flowctl",
			},
		},
		"spec": map[string]interface{}{
			"ports": ports,
			"selector": map[string]interface{}{
				"app.kubernetes.io/name":      pipeline.Metadata.Name,
				"app.kubernetes.io/component": componentType,
				"app.kubernetes.io/instance":  component.ID,
			},
			"type": "ClusterIP",
		},
	}

	// Add custom labels if present
	if len(pipeline.Metadata.Labels) > 0 {
		serviceLabels := service["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		for k, v := range pipeline.Metadata.Labels {
			serviceLabels[k] = v
		}
	}

	// Add custom annotations if present
	if len(pipeline.Metadata.Annotations) > 0 {
		service["metadata"].(map[string]interface{})["annotations"] = pipeline.Metadata.Annotations
	}

	return service
}

// sanitizeResourceName sanitizes a name to be used as a Kubernetes resource name
func sanitizeResourceName(name string) string {
	// Replace any non-alphanumeric character with a dash
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	// Ensure name starts with a letter or number
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || (sanitized[0] >= 'A' && sanitized[0] <= 'Z') ||
		(sanitized[0] >= '0' && sanitized[0] <= '9')) {
		sanitized = "rs-" + sanitized
	}

	// Kubernetes names must be lowercase
	return strings.ToLower(sanitized)
}

// getKubernetesImageRef gets the full image reference with registry prefix if needed
func getKubernetesImageRef(image string, registryPrefix string) string {
	if registryPrefix == "" || strings.Contains(image, "/") {
		return image
	}
	return registryPrefix + "/" + image
}

// mergeKubernetesEnvs merges component-specific and global environment variables
func mergeKubernetesEnvs(componentEnv, globalEnv map[string]string) map[string]string {
	if len(componentEnv) == 0 && len(globalEnv) == 0 {
		return nil
	}

	merged := make(map[string]string)

	// Copy global environment variables
	for k, v := range globalEnv {
		merged[k] = v
	}

	// Add component-specific variables (overriding global ones)
	for k, v := range componentEnv {
		merged[k] = v
	}

	return merged
}