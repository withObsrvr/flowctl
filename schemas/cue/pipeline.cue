// Pipeline schema in CUE
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
	
	// At least one source is required
	sources: [...#Component] & [_, ...] 
	
	// Optional list of processors
	processors?: [...#Component]
	
	// At least one sink is required
	sinks: [...#Component] & [_, ...]
	
	// Optional configuration parameters
	config?: [string]: _
}

// Component represents a pipeline component (source, processor, or sink)
#Component: {
	// Unique identifier for the component
	id: string & !=""
	
	// Optional type of the component
	type?: string
	
	// Container image for the component
	image: string & !=""
	
	// Optional command to run in the container
	command?: [...string]
	
	// Input components - required for processors and sinks
	inputs?: [...string]
	
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
}

// Volume represents a volume mount in a component
#Volume: {
	// Name of the volume
	name: string & !=""
	
	// Mount path in the container
	mountPath: string & !=""
	
	// Optional host path for the volume
	hostPath?: string
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

// Component specific validation rules
#SourceComponent: #Component & {
	// Sources don't have inputs
	inputs?: _|_
}

#ProcessorComponent: #Component & {
	// Processors must have at least one input
	inputs: [_, ...] 
}

#SinkComponent: #Component & {
	// Sinks must have at least one input
	inputs: [_, ...]
}