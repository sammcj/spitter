package main

import (
	"fmt"
	"os"

	"github.com/sammcj/spitter/spitter"
	"github.com/spf13/cobra"
)

func main() {
	var customModelDir string
	var ollamaCommand string

	var rootCmd = &cobra.Command{
		Use:   "spitter [local_model] [remote_server]",
		Short: "Copy local Ollama models to a remote instance",
		Long:  `spitter is a tool to copy local Ollama models to a remote instance, skipping already transferred images.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			config := spitter.SyncConfig{
				LocalModel:     args[0],
				RemoteServer:   args[1],
				CustomModelDir: customModelDir,
				OllamaCommand:  ollamaCommand,
			}
			return spitter.Sync(config)
		},
	}

	// Add flags
	rootCmd.Flags().StringVarP(&customModelDir, "model-dir", "d", "", "Custom Ollama model directory path")
	rootCmd.Flags().StringVarP(&ollamaCommand, "ollama-cmd", "c", "", "Custom Ollama command (e.g., \"docker exec -it ollama ollama\")")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
