package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	natsTransport "github.com/igorrius/tcp-sproxy/internal/infrastructure/transport"
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
	Use:   "nats-proxy-client",
	Short: "NATS TCP Proxy Client",
	Long: `A TCP proxy client that uses NATS as the transport layer.
This client listens on a local port and forwards connections to a remote server via NATS.`,
	RunE: runClient,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nats-proxy-client.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")

	// Client-specific flags
	rootCmd.Flags().String("nats-url", "nats://localhost:4222", "NATS server URL")
	rootCmd.Flags().String("listen-addr", "0.0.0.0:8082", "Address to listen on for incoming connections")
	rootCmd.Flags().String("remote-addr", "redis:6379", "Remote address to proxy to")
	rootCmd.Flags().String("proxy-addr", "proxy-server:8081", "Proxy server address")

	// Bind flags to viper
	viper.BindPFlag("nats.url", rootCmd.Flags().Lookup("nats-url"))
	viper.BindPFlag("client.listen_addr", rootCmd.Flags().Lookup("listen-addr"))
	viper.BindPFlag("client.remote_addr", rootCmd.Flags().Lookup("remote-addr"))
	viper.BindPFlag("client.proxy_addr", rootCmd.Flags().Lookup("proxy-addr"))
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".nats-proxy-client")
	}

	viper.AutomaticEnv()

	// Map environment variables to viper keys
	viper.BindEnv("nats.url", "NATS_URL")
	viper.BindEnv("client.listen_addr", "LISTEN_ADDR")
	viper.BindEnv("client.remote_addr", "REMOTE_ADDR")
	viper.BindEnv("client.proxy_addr", "PROXY_ADDR")
	viper.BindEnv("log.level", "LOG_LEVEL")

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func runClient(cmd *cobra.Command, args []string) error {
	// Setup logging
	level, err := logrus.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logger := logrus.WithField("component", "client")
	logger.Info("Starting NATS TCP Proxy Client")

	natsURL := viper.GetString("nats.url")
	listenAddr := viper.GetString("client.listen_addr")
	remoteAddr := viper.GetString("client.remote_addr")
	proxyAddr := viper.GetString("client.proxy_addr")

	if listenAddr == "" {
		return fmt.Errorf("listen address is required")
	}
	if remoteAddr == "" {
		return fmt.Errorf("remote address is required")
	}
	if proxyAddr == "" {
		return fmt.Errorf("proxy address is required")
	}

	logger.Infof("NATS URL: %s", natsURL)
	logger.Infof("Listening on: %s", listenAddr)
	logger.Infof("Proxying to: %s", remoteAddr)
	logger.Infof("via Proxy Server: %s", proxyAddr)

	// Create a new NATS transport
	transport := natsTransport.NewNATSTransport(logger)
	err = transport.Connect(context.Background(), &natsTransport.NATSConfig{
		URL: natsURL,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer transport.Close()

	// Start listening for incoming connections
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}
	defer listener.Close()

	logger.Infof("Client started successfully, waiting for connections on %s", listenAddr)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					logger.WithError(err).Error("Failed to accept connection")
				}
				continue
			}

			go func(conn net.Conn) {
				defer conn.Close()
				logger.Infof("Accepted connection from %s", conn.RemoteAddr())

				proxyConn, err := transport.Dial(ctx, proxyAddr, remoteAddr)
				if err != nil {
					logger.WithError(err).Errorf("Failed to dial proxy")
					return
				}
				defer proxyConn.Close()

				logger.Infof("Proxy connection established to %s", proxyConn.RemoteAddr())

				errCh := make(chan error, 2)
				go func() {
					_, err := transport.Proxy(conn, proxyConn)
					errCh <- err
				}()
				go func() {
					_, err := transport.Proxy(proxyConn, conn)
					errCh <- err
				}()

				select {
				case err := <-errCh:
					if err != nil {
						logger.WithError(err).Error("Proxy error")
					}
				case <-ctx.Done():
					logger.Info("Context cancelled")
				}
				logger.Infof("Connection from %s closed", conn.RemoteAddr())
			}(conn)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down client...")
	cancel() // Cancel context to stop accepting new connections and close existing ones

	// Give connections time to close gracefully
	time.Sleep(2 * time.Second)

	logger.Info("Client shutdown complete")

	return nil
}
