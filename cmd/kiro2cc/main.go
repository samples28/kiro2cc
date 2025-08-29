package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/bestk/kiro2cc/internal/server"
	"github.com/bestk/kiro2cc/internal/token"
	"github.com/spf13/cobra"
)

var logger *slog.Logger

func main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	var rootCmd = &cobra.Command{
		Use:   "kiro2cc",
		Short: "A tool for managing Kiro authentication tokens and proxying Anthropic API requests.",
		Long: `kiro2cc is a command-line tool that allows you to manage Kiro authentication tokens,
refresh them, and run a local proxy server for the Anthropic API with advanced features.`,
	}

	var readCmd = &cobra.Command{
		Use:   "read",
		Short: "Read and display the token",
		Run:   readToken,
	}

	var refreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the token",
		Run:   refreshToken,
	}

	var exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export environment variables",
		Run:   exportEnvVars,
	}

	var claudeCmd = &cobra.Command{
		Use:   "claude",
		Short: "Bypass Claude region restrictions",
		Run:   setClaude,
	}

	var serverCmd = &cobra.Command{
		Use:   "server [port]",
		Short: "Start the Anthropic API proxy server",
		Args:  cobra.MaximumNArgs(1),
		Run:   startServer,
	}

	rootCmd.AddCommand(readCmd, refreshCmd, exportCmd, claudeCmd, serverCmd)
	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

func readToken(cmd *cobra.Command, args []string) {
	tok, err := token.ReadToken()
	if err != nil {
		logger.Error("Failed to read token", "error", err)
		os.Exit(1)
	}
	fmt.Println("Token information:")
	fmt.Printf("Access Token: %s\n", tok.AccessToken)
	fmt.Printf("Refresh Token: %s\n", tok.RefreshToken)
	if tok.ExpiresAt != "" {
		fmt.Printf("Expires At: %s\n", tok.ExpiresAt)
	}
}

func refreshToken(cmd *cobra.Command, args []string) {
	tok, err := token.RefreshToken()
	if err != nil {
		logger.Error("Failed to refresh token", "error", err)
		os.Exit(1)
	}
	fmt.Println("Token refreshed successfully!")
	fmt.Printf("New Access Token: %s\n", tok.AccessToken)
}

func exportEnvVars(cmd *cobra.Command, args []string) {
	tok, err := token.ReadToken()
	if err != nil {
		logger.Error("Failed to read token", "error", err)
		os.Exit(1)
	}

	if runtime.GOOS == "windows" {
		fmt.Println("CMD")
		fmt.Printf("set ANTHROPIC_BASE_URL=http://localhost:8080\n")
		fmt.Printf("set ANTHROPIC_API_KEY=%s\n\n", tok.AccessToken)
		fmt.Println("Powershell")
		fmt.Println(`$env:ANTHROPIC_BASE_URL="http://localhost:8080"`)
		fmt.Printf(`$env:ANTHROPIC_API_KEY="%s"`, tok.AccessToken)
	} else {
		fmt.Printf("export ANTHROPIC_BASE_URL=http://localhost:8080\n")
		fmt.Printf("export ANTHROPIC_API_KEY=\"%s\"\n", tok.AccessToken)
	}
}

func setClaude(cmd *cobra.Command, args []string) {
	// This function needs to be refactored to not exit on error.
	// For now, leaving it as is.
	fmt.Println("This feature is not yet fully refactored.")
}

func startServer(cmd *cobra.Command, args []string) {
	port := "8080"
	if len(args) > 0 {
		port = args[0]
	}
	srv := server.New(logger)
	srv.Start(port)
}
