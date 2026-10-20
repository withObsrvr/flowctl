package translator

import (
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
