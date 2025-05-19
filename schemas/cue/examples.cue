// Example pipelines in CUE
package pipeline

// A minimal valid pipeline
minimalPipeline: #Pipeline & {
	apiVersion: "flowctl/v1"
	kind: "Pipeline"
	metadata: {
		name: "minimal-pipeline"
	}
	spec: {
		sources: [{
			id: "mock-source"
			image: "mock-source:latest"
		}]
		sinks: [{
			id: "stdout-sink"
			image: "stdout-sink:latest"
			inputs: ["mock-source"]
		}]
	}
}

// A more complete pipeline example
completePipeline: #Pipeline & {
	apiVersion: "flowctl/v1"
	kind: "Pipeline"
	metadata: {
		name: "stellar-token-pipeline"
		namespace: "default"
		labels: {
			app: "token-tracker"
			environment: "production"
		}
		annotations: {
			"description": "Monitors Stellar token transactions"
		}
	}
	spec: {
		description: "Pipeline for processing Stellar token transfers"
		driver: "kubernetes"
		sources: [{
			id: "stellar-live-source"
			type: "source"
			image: "ghcr.io/withobsrvr/stellar-live-source-datalake:latest"
			config: {
				network: "testnet"
				start_ledger: 1234567
			}
			health_check: "/healthz"
			health_port: 8080
			resources: {
				requests: {
					cpu: "100m"
					memory: "256Mi"
				}
				limits: {
					cpu: "500m"
					memory: "512Mi"
				}
			}
		}]
		processors: [{
			id: "ttp-processor"
			image: "ghcr.io/withobsrvr/ttp-processor:latest"
			inputs: ["stellar-live-source"]
			env: {
				LOG_LEVEL: "debug"
				BUFFER_SIZE: "1000"
			}
			replicas: 2
		}]
		sinks: [{
			id: "consumer-app"
			image: "ghcr.io/withobsrvr/consumer-app:latest"
			inputs: ["ttp-processor"]
			volumes: [{
				name: "data-volume"
				mountPath: "/data"
			}]
			ports: [{
				name: "http"
				containerPort: 8080
				hostPort: 18080
				protocol: "TCP"
			}]
		}]
	}
}

// An invalid pipeline example (with validation errors)
invalidPipeline: #Pipeline & {
	apiVersion: "flowctl/v1"
	kind: "Pipeline"
	metadata: {
		name: "invalid-pipeline"
	}
	spec: {
		driver: "unsupported"  // Invalid driver value
		sources: [{
			id: "mock-source"
			image: "mock-source:latest"
			health_check: "/health"  // Missing health_port
		}]
		processors: [{
			id: "processor"
			image: "processor:latest"
			// Missing inputs (required for processors)
			replicas: -1  // Invalid negative value
		}]
		sinks: [{
			id: "sink"
			image: "sink:latest"
			inputs: ["unknown-component"]  // Invalid input reference
		}]
	}
}