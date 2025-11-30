// Unified Pipeline schema in CUE with comprehensive validation
package pipeline

// Pipeline represents a data processing pipeline with all components
#Pipeline: {
	// API version for the pipeline format
	apiVersion: string & =~"^flowctl/v[0-9]+$"
	
	// Kind of resource - must be "Pipeline"
	kind: "Pipeline"
	
	// Metadata for the pipeline
	metadata: #Metadata
	
	// Specification of the pipeline
	spec: #Spec
}

// Metadata contains pipeline metadata
#Metadata: {
	// Name of the pipeline
	name: string & !=""
	
	// Optional namespace for the pipeline
	namespace?: string
	
	// Optional labels as key-value pairs
	labels?: [string]: string
	
	// Optional annotations as key-value pairs
	annotations?: [string]: string
}

// Spec contains the pipeline specification
#Spec: {
	// Optional description of the pipeline
	description?: string
	
	// Optional driver for deployment
	driver?: "docker" | "kubernetes" | "k8s" | "nomad" | "local"
	
	// Sources - can be empty if pipelines are present
	sources: [...#SourceComponent]
	
	// Optional list of processors - use ProcessorComponent to enforce processor-specific rules
	processors?: [...#ProcessorComponent]
	
	// Sinks - can be empty if pipelines are present
	sinks: [...#SinkComponent]
	
	// Optional list of complete pipelines as components
	pipelines?: [...#PipelineComponent]
	
	// Optional configuration parameters
	config?: [string]: _
	
	// Validation: must have at least one source or pipeline component
	_hasComponents: len(sources) > 0 || (pipelines != _|_ && len(pipelines) > 0)
	_hasComponents: true
}

// Base component definition with common fields
#BaseComponent: {
	// Unique identifier for the component
	id: string & !=""

	// Optional type of the component
	type?: string

	// Container image for the component (alternative to command)
	image?: string & !=""

	// Command to run (alternative to image)
	command?: [...string] & [_, ...]

	// Optional arguments (used with both image and command)
	args?: [...string]

	// Validation: must have either image or command (but not both)
	_hasImage: image != _|_
	_hasCommand: command != _|_
	_validExecution: (_hasImage && !_hasCommand) || (!_hasImage && _hasCommand)
	_validExecution: true

	// Optional configuration parameters
	config?: [string]: _
	
	// Optional environment variables
	env?: [string]: string
	
	// Optional volume mounts
	volumes?: [...#Volume]
	
	// Optional ports to expose
	ports?: [...#Port]
	
	// Optional health check endpoint
	health_check?: string
	
	// Health port - required if health_check is specified
	health_port?: int & >0
	
	// Optional number of replicas
	replicas?: int & >=0
	
	// Optional resource requirements
	resources?: #Resources
	
	// Validation rule: if health_check is present, health_port must also be present
	if health_check != _|_ {
		health_port: int & >0
	}
	
	// Validation rule: replicas must be non-negative
	if replicas != _|_ {
		replicas: int & >=0
	}
}

// Source component (doesn't have inputs)
#SourceComponent: #BaseComponent & {
	// Sources don't have inputs
	inputs?: _|_
}

// Processor component (requires inputs)
#ProcessorComponent: #BaseComponent & {
	// Processors must have at least one input
	inputs: [...string] & [_, ...] 
}

// Sink component (requires inputs)
#SinkComponent: #BaseComponent & {
	// Sinks must have at least one input
	inputs: [...string] & [_, ...] 
}

// Pipeline component (complete pipelines that run as black boxes)
#PipelineComponent: #BaseComponent & {
	// Pipeline type must be "pipeline"
	type: "pipeline"
	
	// Optional command arguments
	args?: [...string]
	
	// Optional dependencies on other pipelines
	depends_on?: [...string]
	
	// Optional restart policy
	restart_policy?: "no" | "always" | "on-failure" | "unless-stopped"
}

// Volume represents a volume mount in a component
#Volume: {
	// Name of the volume
	name: string & !=""
	
	// Mount path in the container
	mountPath: string & !=""
	
	// Optional host path for the volume
	hostPath?: string
	
	// Alternative container path (takes precedence over mountPath)
	container_path?: string
	
	// Optional read-only flag
	readonly?: bool
}

// Port represents a port exposed by a component
#Port: {
	// Optional name for the port
	name?: string
	
	// Container port to expose
	containerPort: int & >0 & <65536
	
	// Optional host port mapping
	hostPort?: int & >0 & <65536
	
	// Optional protocol (defaults to TCP)
	protocol?: "TCP" | "UDP" | "SCTP"
}

// Resources represents compute resources requests and limits
#Resources: {
	// Optional resource requests
	requests?: #ResourceList
	
	// Optional resource limits
	limits?: #ResourceList
}

// ResourceList is a set of resource requests or limits
#ResourceList: {
	// CPU resource (e.g., "100m", "0.5", "1")
	cpu?: string & =~"^([0-9]+(\\.[0-9]+)?(m)?)?$"
	
	// Memory resource (e.g., "64Mi", "2Gi")
	memory?: string & =~"^([0-9]+(.[0-9]+)?(Ki|Mi|Gi|Ti|Pi|Ei)?)?$"
}