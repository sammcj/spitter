package spitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/schollz/progressbar/v3"
)

type SyncConfig struct {
	LocalModel   string
	RemoteServer string
}

type Layer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

type Manifest struct {
	Layers []Layer `json:"layers"`
}

func Sync(config SyncConfig) error {
	if !validateURL(config.RemoteServer) {
		return fmt.Errorf("invalid remote server URL: %s", config.RemoteServer)
	}

	baseDir, err := getOllamaModelsDir()
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

	var modelFrom string
	for _, layer := range manifest.Layers {
		if strings.HasPrefix(layer.MediaType, "application/vnd.ollama.image.model") ||
			strings.HasPrefix(layer.MediaType, "application/vnd.ollama.image.projector") ||
			strings.HasPrefix(layer.MediaType, "application/vnd.ollama.image.adapter") {
			hash := layer.Digest[7:]
			if err := uploadLayer(config.RemoteServer, blobDir, hash); err != nil {
				return err
			}
			modelFrom += fmt.Sprintf("FROM @sha256:%s\n", hash)
		}
	}

	modelfile, err := getModelfile(config.LocalModel)
	if err != nil {
		return err
	}

	modelfile = modelFrom + modelfile

	return createModel(config.RemoteServer, config.LocalModel, modelfile)
}

func validateURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func getOllamaModelsDir() (string, error) {
	ollamaModels := os.Getenv("OLLAMA_MODELS")
	if ollamaModels != "" && ollamaModels != "*" {
		return ollamaModels, nil
	}

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
	resp, err := http.Head(fmt.Sprintf("%s/api/blobs/sha256:%s", remoteServer, hash))
	if err != nil {
		return err
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
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	bar := progressbar.DefaultBytes(
		stat.Size(),
		"Uploading",
	)

	resp, err = http.Post(fmt.Sprintf("%s/api/blobs/sha256:%s", remoteServer, hash), "application/octet-stream", io.TeeReader(file, bar))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed: %s", resp.Status)
	}

	fmt.Println("Success uploading layer.")
	return nil
}

func getModelfile(modelName string) (string, error) {
	cmd := exec.Command("ollama", "show", modelName, "--modelfile")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not get ollama Modelfile: %w", err)
	}

	return parseModelfile(string(output)), nil
}

func parseModelfile(input string) string {
	lines := strings.Split(input, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "FROM ") && !strings.HasPrefix(line, "failed to get console mode") {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func createModel(remoteServer, modelName, modelfile string) error {
	modelCreate := struct {
		Name      string `json:"name"`
		Modelfile string `json:"modelfile"`
	}{
		Name:      modelName,
		Modelfile: modelfile,
	}

	data, err := json.Marshal(modelCreate)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/create", remoteServer), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not create %s on the remote server (%d): %s", modelName, resp.StatusCode, resp.Status)
	}

	fmt.Println("Model created successfully on the remote server.")
	return nil
}
