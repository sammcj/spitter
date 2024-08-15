# Spitter

Spitter is a Go package and command-line tool for copying local Ollama models to a remote instance.
Spitter will skip already transferred images.

## Features

- Copy local Ollama models to a remote server
- Skip already transferred images
- Upload at high speed with a progress bar
- Ideal for servers isolated from the internet

## Installation

To install the command-line tool, run:

```shell
go install github.com/sammcj/spitter/cmd/spitter@HEAD
```

## Usage

### As a command-line tool

```shell
spitter [local_model] [remote_server]
```

Example:

```shell
spitter modelname http://192.168.0.100:11434
```

### As a Go package

```go
import "github.com/sammcj/spitter/spitter"

config := spitter.SyncConfig{
    LocalModel:   "modelname",
    RemoteServer: "http://192.168.0.100:11434",
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
