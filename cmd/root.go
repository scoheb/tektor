package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/lcarva/tektor/cmd/validate"
)

var rootCmd = &cobra.Command{
	Use:          "tektor",
	Short:        "Tektor is a validator for Tekton resources.",
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		// Check if it's an unsupported resource error
		var unsupportedErr validate.UnsupportedResourceError
		if errors.As(err, &unsupportedErr) {
			// Print the message and exit with code 2 for unsupported resources
			os.Stderr.WriteString(unsupportedErr.Message + "\n")
			os.Exit(2)
		}
		// For all other errors, exit with code 1
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(validate.ValidateCmd)
}
