package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/services"
	"github.com/igorrius/tcp-sproxy/internal/infrastructure/repositories"
	"github.com/igorrius/tcp-sproxy/internal/infrastructure/transport"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	logLevel string
)

func main() {
	Execute()
}

var rootCmd = &cobra.Command{
	Use:   "nats-proxy-server",
	Short: "NATS TCP Proxy Server",
	Long: `A TCP proxy server that uses NATS as the transport layer.
This server accepts connections and forwards them to a remote host using NATS.`,
	RunE: runServer,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nats-proxy-server.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")

	// Server-specific flags
	rootCmd.Flags().String("nats-url", "nats://localhost:4222", "NATS server URL")
	rootCmd.Flags().String("listen-addr", ":8080", "Address to listen on for incoming connections")

	// Bind flags to viper
	viper.BindPFlag("nats.url", rootCmd.Flags().Lookup("nats-url"))
	viper.BindPFlag("server.listen_addr", rootCmd.Flags().Lookup("listen-addr"))
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".nats-proxy-server")
	}

	viper.AutomaticEnv()

	// Map environment variables to viper keys
	viper.BindEnv("nats.url", "NATS_URL")
	viper.BindEnv("log.level", "LOG_LEVEL")

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Setup logging
	level, err := logrus.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logger := logrus.WithField("component", "server")
	logger.Info("Starting NATS TCP Proxy Server")

	// Server now receives remote host/port from client messages

	// Initialize components
	connectionRepo := repositories.NewInMemoryConnectionRepository()
	natsTransport := transport.NewNATSTransport(logger)

	// Connect to NATS
	natsConfig := &transport.NATSConfig{
		URL:     viper.GetString("nats.url"),
		Timeout: 30 * time.Second,
	}

	if err := natsTransport.Connect(context.Background(), natsConfig); err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer natsTransport.Close()

	proxyService := services.NewProxyService(connectionRepo, natsTransport, logger)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start listening for proxy requests
	go func() {
		err := proxyService.HandleNATSProxyRequests(ctx, "proxy.request")
		if err != nil {
			logger.WithError(err).Error("Failed to handle NATS proxy requests")
			cancel() // Stop the application if request handling fails
		}
	}()

	logger.Info("Server started successfully - listening for NATS messages")

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down server...")
	cancel() // Cancel context to stop services

	// Give services time to gracefully shutdown
	time.Sleep(2 * time.Second)

	logger.Info("Server shutdown complete")

	return nil
}
