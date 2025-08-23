package spitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

type SyncConfig struct {
	LocalModel      string
	RemoteServer    string
	CustomModelDir  string
	OllamaCommand   string // Custom Ollama command (e.g., "docker exec -it ollama ollama")
	AllModels       bool   // Flag to push all models instead of a single one
}

type Layer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

type Manifest struct {
	Layers []Layer `json:"layers"`
}

// listModels returns a list of all available models in the Ollama models directory
func listModels(baseDir string) ([]string, error) {
	manifestsRoot := filepath.Join(baseDir, "manifests")
	var models []string

	if _, err := os.Stat(manifestsRoot); os.IsNotExist(err) {
		return models, nil // No manifests directory, so no models.
	}

	err := filepath.WalkDir(manifestsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(manifestsRoot, path)
		if err != nil {
			return err
		}

		parts := strings.Split(relPath, string(os.PathSeparator))
		if len(parts) < 2 {
			return nil // Not enough parts for a model name
		}

		var modelName string
		// The last part is the tag
		tag := parts[len(parts)-1]
		// The parts before the tag form the model path
		modelPathParts := parts[:len(parts)-1]

		// Handle different repository structures
		if modelPathParts[0] == "registry.ollama.ai" {
			// Path: registry.ollama.ai/library/model/version or registry.ollama.ai/namespace/model/version
			if len(modelPathParts) > 2 && modelPathParts[1] == "library" {
				// It's a library model, name is just "model"
				modelName = strings.Join(modelPathParts[2:], "/")
			} else if len(modelPathParts) > 1 {
				// It's a namespaced model, name is "namespace/model"
				modelName = strings.Join(modelPathParts[1:], "/")
			}
		} else {
			// Other registries or hub models. The full path is the model name.
			// e.g., hub/user/model
			modelName = strings.Join(modelPathParts, "/")
		}

		if modelName != "" {
			models = append(models, fmt.Sprintf("%s:%s", modelName, tag))
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking models directory: %w", err)
	}

	return models, nil
}

// syncSingleModel syncs a single model to the remote server
func syncSingleModel(config SyncConfig) error {
	baseDir, err := getOllamaModelsDir(config.CustomModelDir)
	if err != nil {
		return err
	}

	blobDir := filepath.Join(baseDir, "blobs")
	modelDir := filepath.Join(baseDir, "manifests", config.LocalModel)
	manifestFile := strings.Replace(config.LocalModel, ":", string(os.PathSeparator), 1)

	if modelBase(config.LocalModel) == "hub" {
		modelDir = filepath.Join(baseDir, "manifests", manifestFile)
	} else if modelBase(config.LocalModel) == "" {
		modelDir = filepath.Join(baseDir, "manifests", "registry.ollama.ai", "library", manifestFile)
	} else {
		modelDir = filepath.Join(baseDir, "manifests", "registry.ollama.ai", manifestFile)
	}

	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		return fmt.Errorf("model not found in %s", modelDir)
	}

	manifestData, err := os.ReadFile(modelDir)
	if err != nil {
		return err
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return err
	}

	fmt.Printf("Copying model %s to %s...\n", config.LocalModel, config.RemoteServer)

	// Upload all the layers
	for _, layer := range manifest.Layers {
		if strings.HasPrefix(layer.MediaType, "application/vnd.ollama.image.model") ||
			strings.HasPrefix(layer.MediaType, "application/vnd.ollama.image.projector") ||
			strings.HasPrefix(layer.MediaType, "application/vnd.ollama.image.adapter") {
			hash := layer.Digest[7:]
			if err := uploadLayer(config.RemoteServer, blobDir, hash); err != nil {
				return err
			}
		}
	}

	// Get the original modelfile
	modelfile, err := getModelfile(config.LocalModel, config.OllamaCommand)
	if err != nil {
		return err
	}

	// fmt.Println("Final Modelfile content:")
	// fmt.Println("------------------------")
	// fmt.Println(modelfile)
	// fmt.Println("------------------------")

	return createModel(config.RemoteServer, config.LocalModel, modelfile)
}

func Sync(config SyncConfig) error {
	if !validateURL(config.RemoteServer) {
		return fmt.Errorf("invalid remote server URL: %s", config.RemoteServer)
	}

	// If AllModels flag is set, sync all models
	if config.AllModels {
		baseDir, err := getOllamaModelsDir(config.CustomModelDir)
		if err != nil {
			return err
		}

		models, err := listModels(baseDir)
		if err != nil {
			return fmt.Errorf("error listing models: %w", err)
		}

		if len(models) == 0 {
			return fmt.Errorf("no models found in %s", baseDir)
		}

		fmt.Printf("Found %d models to sync\n", len(models))

		var syncErrors []string
		for _, model := range models {
			fmt.Printf("\n=== Syncing model: %s ===\n", model)
			modelConfig := config
			modelConfig.LocalModel = model

			err := syncSingleModel(modelConfig)
			if err != nil {
				errMsg := fmt.Sprintf("Error syncing model %s: %v", model, err)
				syncErrors = append(syncErrors, errMsg)
				fmt.Println(errMsg)
				// Continue with other models even if one fails
				continue
			}
		}

		if len(syncErrors) > 0 {
			fmt.Printf("\n=== Sync completed with %d errors ===\n", len(syncErrors))
			for _, errMsg := range syncErrors {
				fmt.Println(errMsg)
			}
			return fmt.Errorf("%d models failed to sync", len(syncErrors))
		}

		fmt.Printf("\n=== Successfully synced all %d models ===\n", len(models))
		return nil
	}

	// Otherwise, sync the single specified model
	return syncSingleModel(config)
}

func validateURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func getOllamaModelsDir(customDir string) (string, error) {
	// If a custom directory is provided, use it
	if customDir != "" {
		return customDir, nil
	}

	// Otherwise check environment variable
	ollamaModels := os.Getenv("OLLAMA_MODELS")
	if ollamaModels != "" && ollamaModels != "*" {
		return ollamaModels, nil
	}

	// Fall back to default paths
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("USERPROFILE"), ".ollama", "models"), nil
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), ".ollama", "models"), nil
	default:
		return "/usr/share/ollama/.ollama/models", nil
	}
}

func modelBase(modelName string) string {
	parts := strings.SplitN(modelName, "/", 2)
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

func uploadLayer(remoteServer, blobDir, hash string) error {
	// First check if the blob already exists on the remote server
	checkURL := fmt.Sprintf("%s/api/blobs/sha256:%s", remoteServer, hash)
	resp, err := http.Head(checkURL)
	if err != nil {
		return fmt.Errorf("error checking if blob exists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Skipping upload for already created layer sha256:%s\n", hash)
		return nil
	}

	fmt.Printf("Uploading layer sha256:%s\n", hash)
	blobFile := filepath.Join(blobDir, fmt.Sprintf("sha256-%s", hash))

	file, err := os.Open(blobFile)
	if err != nil {
		return fmt.Errorf("error opening blob file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting file stats: %w", err)
	}

	bar := progressbar.DefaultBytes(
		stat.Size(),
		"Uploading",
	)

	// Create a new HTTP client with a longer timeout
	client := &http.Client{
		Timeout: 30 * time.Minute, // Set a long timeout for large model uploads
	}

	// Create a new request
	uploadURL := fmt.Sprintf("%s/api/blobs/sha256:%s", remoteServer, hash)
	req, err := http.NewRequest("POST", uploadURL, io.TeeReader(file, bar))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	// Execute the request
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("error uploading blob: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	fmt.Println("Success uploading layer.")
	return nil
}

func getModelfile(modelName string, ollamaCommand string) (string, error) {
	var cmd *exec.Cmd
	var err error
	var output []byte

	// Try with the provided custom command if specified
	if ollamaCommand != "" {
		fmt.Printf("Using custom Ollama command: %s\n", ollamaCommand)
		parts := strings.Fields(ollamaCommand)
		if len(parts) == 0 {
			return "", fmt.Errorf("invalid ollama command: %s", ollamaCommand)
		}

		args := append(parts[1:], "show", modelName, "--modelfile")
		cmd = exec.Command(parts[0], args...)
		output, err = cmd.CombinedOutput()
		if err == nil {
			return parseModelfile(string(output)), nil
		}
		fmt.Printf("Custom command failed: %v\n", err)
	}

	// Try with the local ollama binary
	fmt.Println("Trying local ollama binary...")
	cmd = exec.Command("ollama", "show", modelName, "--modelfile")
	output, err = cmd.CombinedOutput()
	if err == nil {
		return parseModelfile(string(output)), nil
	}

	// Check if the error is "executable file not found"
	if exitErr, ok := err.(*exec.Error); ok && exitErr.Err == exec.ErrNotFound {
		fmt.Println("Local ollama binary not found, checking for Docker container...")

		// Check if there's a Docker container named 'ollama' running
		dockerCmd := exec.Command("docker", "ps", "--filter", "name=ollama", "--format", "{{.Names}}")
		dockerOutput, dockerErr := dockerCmd.CombinedOutput()
		if dockerErr == nil && strings.Contains(string(dockerOutput), "ollama") {
			fmt.Println("Found ollama Docker container, using docker exec...")

			// Use docker exec to run the ollama command
			dockerExecCmd := exec.Command("docker", "exec", "ollama", "ollama", "show", modelName, "--modelfile")
			dockerExecOutput, dockerExecErr := dockerExecCmd.CombinedOutput()
			if dockerExecErr == nil {
				return parseModelfile(string(dockerExecOutput)), nil
			}
			fmt.Printf("Docker exec command failed: %v\n", dockerExecErr)
		} else {
			fmt.Printf("No ollama Docker container found or docker command failed: %v\n", dockerErr)
		}
	}

	// If all attempts fail, try to extract the Modelfile from the manifest
	fmt.Println("All attempts to get Modelfile failed, trying to extract from manifest...")
	modelfile, extractErr := extractModelfileFromManifest(modelName)
	if extractErr != nil {
		return "", fmt.Errorf("could not get ollama Modelfile: %w (and fallback extraction failed: %v)", err, extractErr)
	}

	return modelfile, nil
}

// extractModelfileFromManifest attempts to extract the Modelfile content from the model's manifest
// This is used as a fallback when the ollama CLI is not available
func extractModelfileFromManifest(modelName string) (string, error) {
	return fmt.Sprintf("# Modelfile for %s\n", modelName), nil
}

func parseModelfile(input string) string {
	lines := strings.Split(input, "\n")
	var filtered []string
	for _, line := range lines {
		// Filter out comments and error messages, but keep FROM statements
		if !strings.HasPrefix(line, "#") &&
		   !strings.HasPrefix(line, "failed to get console mode") {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func createModel(remoteServer, modelName, modelfile string) error {
	// Check if the model already exists on the remote server
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// First check if the model exists
	checkURL := fmt.Sprintf("%s/api/show?name=%s", remoteServer, url.QueryEscape(modelName))
	fmt.Printf("Checking if model exists at %s\n", checkURL)

	resp, err := client.Get(checkURL)
	if err != nil {
		fmt.Printf("Error checking if model exists: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("Model %s already exists on the remote server, will update it\n", modelName)
		} else {
			fmt.Printf("Model %s does not exist on the remote server (status: %d), will create it\n", modelName, resp.StatusCode)
		}
	}

	// Parse the modelfile to extract parameters
	template, system, parameters := parseModelfileParams(modelfile)

	// Collect all the layer hashes from the modelfile
	var files map[string]string
	files = make(map[string]string)

	// Extract FROM statements to get the layer hashes
	lines := strings.Split(modelfile, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				fromValue := parts[1]
				if strings.HasPrefix(fromValue, "@sha256:") {
					hash := fromValue[8:] // Remove the "@sha256:" prefix
					files[fmt.Sprintf("%s.gguf", hash[:8])] = fmt.Sprintf("sha256:%s", hash)
				} else if strings.HasPrefix(fromValue, "/") {
					// Handle absolute path to blob file
					baseName := filepath.Base(fromValue)
					if strings.HasPrefix(baseName, "sha256-") {
						hash := baseName[7:] // Remove the "sha256-" prefix
						files[fmt.Sprintf("%s.gguf", hash[:8])] = fmt.Sprintf("sha256:%s", hash)
					}
				}
			}
		}
	}

	// Create the model creation request
	type ModelCreateRequest struct {
		Model      string            `json:"model"`
		Files      map[string]string `json:"files,omitempty"`
		Template   string            `json:"template,omitempty"`
		System     string            `json:"system,omitempty"`
		Parameters map[string]string `json:"parameters,omitempty"`
	}

	modelCreate := ModelCreateRequest{
		Model:      modelName,
		Files:      files,
		Template:   template,
		System:     system,
		Parameters: parameters,
	}

	data, err := json.Marshal(modelCreate)
	if err != nil {
		return err
	}

	fmt.Printf("Sending model creation request to %s/api/create\n", remoteServer)
	fmt.Printf("Model name: %s\n", modelName)

	// Create a new HTTP client with a longer timeout
	client = &http.Client{
		Timeout: 5 * time.Minute, // Set a timeout for model creation
	}

	// Create a new request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/create", remoteServer), bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("error creating model: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not create %s on the remote server (%d): %s - %s",
			modelName, resp.StatusCode, resp.Status, string(body))
	}

	fmt.Println("Model created successfully on the remote server.")
	fmt.Printf("Response: %s\n", string(body))

	return nil
}

// parseModelfileParams extracts template, system, and parameters from a modelfile
func parseModelfileParams(modelfile string) (string, string, map[string]string) {
	var template, system string
	parameters := make(map[string]string)

	lines := strings.Split(modelfile, "\n")
	inTemplate := false
	templateLines := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if inTemplate {
			if strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, `'''`) {
				inTemplate = false
				template = strings.Join(templateLines, "\n")
			} else {
				templateLines = append(templateLines, line)
			}
			continue
		}

		if strings.HasPrefix(line, "TEMPLATE ") {
			inTemplate = true
			continue
		}

		if strings.HasPrefix(line, "SYSTEM ") {
			system = strings.TrimPrefix(line, "SYSTEM ")
			// Remove quotes if present
			if len(system) >= 2 && ((system[0] == '"' && system[len(system)-1] == '"') ||
			                        (system[0] == '\'' && system[len(system)-1] == '\'')) {
				system = system[1 : len(system)-1]
			}
			continue
		}

		if strings.HasPrefix(line, "PARAMETER ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				paramName := parts[1]
				paramValue := parts[2]
				parameters[paramName] = paramValue
			}
		}
	}

	return template, system, parameters
}
