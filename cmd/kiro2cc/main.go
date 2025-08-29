package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/bestk/kiro2cc/internal/config"
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
		Use:   "claude [region]",
		Short: "Bypass Claude region restrictions by setting the AWS region (e.g., us-east-1, eu-west-2)",
		Args:  cobra.ExactArgs(1),
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
	region := args[0]
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	cfg.Region = region
	if err := config.SaveConfig(cfg); err != nil {
		logger.Error("Failed to save config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Claude region set to: %s\n", region)
}

func startServer(cmd *cobra.Command, args []string) {
	port := "8080"
	if len(args) > 0 {
		port = args[0]
	}
	srv := server.New(logger)
	srv.Start(port)
}
