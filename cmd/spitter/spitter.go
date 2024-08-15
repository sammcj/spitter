package main

import (
	"fmt"
	"os"

	"github.com/sammcj/spitter"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "spitter [local_model] [remote_server]",
		Short: "Copy local Ollama models to a remote instance",
		Long:  `spitter is a tool to copy local Ollama models to a remote instance, skipping already transferred images and uploading at high speed with a progress bar.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			config := spitter.SyncConfig{
				LocalModel:   args[0],
				RemoteServer: args[1],
			}
			return spitter.Sync(config)
		},
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
