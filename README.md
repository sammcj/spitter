# Spitter

Spitter is a Go package and command-line tool for copying local Ollama models to a remote instance.
Spitter will skip already transferred images.

## Features

- Copy local Ollama models to a remote server
- Skip already transferred images
- Upload at high speed with a progress bar
- Ideal for servers isolated from the internet
- Automatic detection of Ollama in Docker containers
- Fallback mechanisms when Ollama CLI is not available

## Installation

To install the command-line tool, run:

```shell
go install github.com/sammcj/spitter/cmd/spitter@HEAD
```

## Usage

### As a command-line tool

```shell
spitter [local_model] [remote_server] [flags]
```

Flags:

- `-a, --all` : Push all models to the remote host
- `-d, --model-dir string` : Custom Ollama model directory path
- `-c, --ollama-cmd string` : Custom Ollama command (e.g., "docker exec -it ollama ollama")

Examples:

```shell
# Basic usage
spitter modelname http://192.168.0.100:11434

# Push all models to the remote host
spitter modelname http://192.168.0.100:11434 --all

# With custom model directory
spitter modelname http://192.168.0.100:11434 --model-dir /path/to/custom/models

# With custom Ollama command for Docker
spitter modelname http://192.168.0.100:11434 --ollama-cmd "docker exec -it ollama ollama"

# With both custom model directory and Ollama command
spitter modelname http://192.168.0.100:11434 --model-dir /path/to/custom/models --ollama-cmd "docker exec -it ollama ollama"

# Push all models with custom model directory
spitter modelname http://192.168.0.100:11434 --all --model-dir /path/to/custom/models
```

### As a Go package

```go
import "github.com/sammcj/spitter/spitter"

config := spitter.SyncConfig{
    LocalModel:     "modelname",
    RemoteServer:   "http://192.168.0.100:11434",
    CustomModelDir: "/path/to/custom/models", // Optional: custom Ollama model directory
    OllamaCommand:  "docker exec ollama ollama", // Optional: custom Ollama command
}

err := spitter.Sync(config)
if err != nil {
    // Handle error
}
```

## Requirements

- Go 1.21 or later
- Ollama installed on both local and remote machines

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

- Copyright 2024 Sam McLeod
- This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
