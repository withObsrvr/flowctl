package translator

import (
	"fmt"
	"os"
	"path/filepath"
)

// ensureDir ensures that the directory exists
func ensureDir(dir string) error {
	// Create the directory if it doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// createFile creates a new file with the given data
func createFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// writeToFile writes data to a file
func writeToFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := ensureDir(dir); err != nil {
		return err
	}

	return createFile(path, data)
}

// findPort finds a port in a slice of ports that matches the format
func findPort(ports []int, basePort int) int {
	if len(ports) == 0 {
		return basePort
	}
	return ports[0]
}

// generatePortMappings generates port mappings for Docker/K8s
func generatePortMappings(ports []int) []string {
	if len(ports) == 0 {
		return nil
	}
	
	mappings := make([]string, len(ports))
	for i, p := range ports {
		mappings[i] = fmt.Sprintf("%d:%d", p, p)
	}
	return mappings
}

// mergeMaps merges two string maps with the second taking precedence
func mergeMaps(m1, m2 map[string]string) map[string]string {
	result := make(map[string]string)
	
	// Copy first map
	for k, v := range m1 {
		result[k] = v
	}
	
	// Merge second map
	for k, v := range m2 {
		result[k] = v
	}
	
	return result
}

// getComponentName extracts a standardized component name
func getComponentName(componentType, name string, prefix string) string {
	baseName := name
	if baseName == "" {
		baseName = componentType
	}
	
	if prefix != "" {
		return fmt.Sprintf("%s-%s", prefix, baseName)
	}
	return baseName
}

// extractParam extracts a parameter from a map as a certain type
func extractParam(params map[string]interface{}, key string, defaultValue interface{}) interface{} {
	if val, ok := params[key]; ok {
		return val
	}
	return defaultValue
}

// extractStringParam extracts a string parameter from a map
func extractStringParam(params map[string]interface{}, key, defaultValue string) string {
	val := extractParam(params, key, defaultValue)
	if strVal, ok := val.(string); ok {
		return strVal
	}
	return fmt.Sprintf("%v", val)
}

// extractIntParam extracts an int parameter from a map
func extractIntParam(params map[string]interface{}, key string, defaultValue int) int {
	val := extractParam(params, key, defaultValue)
	if intVal, ok := val.(int); ok {
		return intVal
	}
	if strVal, ok := val.(string); ok {
		var intVal int
		if _, err := fmt.Sscanf(strVal, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// extractBoolParam extracts a boolean parameter from a map
func extractBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	val := extractParam(params, key, defaultValue)
	if boolVal, ok := val.(bool); ok {
		return boolVal
	}
	if strVal, ok := val.(string); ok {
		if strVal == "true" {
			return true
		} else if strVal == "false" {
			return false
		}
	}
	return defaultValue
}

// extractStringMapParam extracts a string map parameter from a map
func extractStringMapParam(params map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	
	val := extractParam(params, key, nil)
	if val == nil {
		return result
	}
	
	if mapVal, ok := val.(map[string]interface{}); ok {
		for k, v := range mapVal {
			if strVal, ok := v.(string); ok {
				result[k] = strVal
			} else {
				result[k] = fmt.Sprintf("%v", v)
			}
		}
	} else if mapVal, ok := val.(map[string]string); ok {
		for k, v := range mapVal {
			result[k] = v
		}
	}
	
	return result
}

// extractStringSliceParam extracts a string slice parameter from a map
func extractStringSliceParam(params map[string]interface{}, key string) []string {
	val := extractParam(params, key, nil)
	if val == nil {
		return nil
	}
	
	if sliceVal, ok := val.([]interface{}); ok {
		result := make([]string, len(sliceVal))
		for i, v := range sliceVal {
			if strVal, ok := v.(string); ok {
				result[i] = strVal
			} else {
				result[i] = fmt.Sprintf("%v", v)
			}
		}
		return result
	} else if sliceVal, ok := val.([]string); ok {
		return sliceVal
	}
	
	return []string{fmt.Sprintf("%v", val)}
}

// extractIntSliceParam extracts an int slice parameter from a map
func extractIntSliceParam(params map[string]interface{}, key string) []int {
	val := extractParam(params, key, nil)
	if val == nil {
		return nil
	}
	
	if sliceVal, ok := val.([]interface{}); ok {
		result := make([]int, 0, len(sliceVal))
		for _, v := range sliceVal {
			if intVal, ok := v.(int); ok {
				result = append(result, intVal)
			} else if strVal, ok := v.(string); ok {
				var intVal int
				if _, err := fmt.Sscanf(strVal, "%d", &intVal); err == nil {
					result = append(result, intVal)
				}
			} else {
				// Try to convert to int
				var intVal int
				if _, err := fmt.Sscanf(fmt.Sprintf("%v", v), "%d", &intVal); err == nil {
					result = append(result, intVal)
				}
			}
		}
		return result
	} else if sliceVal, ok := val.([]int); ok {
		return sliceVal
	}
	
	// Try to convert a single value to int
	if intVal, ok := val.(int); ok {
		return []int{intVal}
	} else if strVal, ok := val.(string); ok {
		var intVal int
		if _, err := fmt.Sscanf(strVal, "%d", &intVal); err == nil {
			return []int{intVal}
		}
	}
	
	return nil
}