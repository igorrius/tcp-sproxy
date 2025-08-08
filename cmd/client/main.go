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
	"github.com/igorrius/tcp-sproxy/pkg/transport"
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
	rootCmd.Flags().Int("local-port", 0, "Local port to listen on")
	rootCmd.Flags().String("listen-addr", "localhost", "Local address to listen on")
	rootCmd.Flags().String("remote-host", "", "Remote host to proxy to")
	rootCmd.Flags().Int("remote-port", 0, "Remote port to proxy to")

	// Bind flags to viper
	viper.BindPFlag("nats.url", rootCmd.Flags().Lookup("nats-url"))
	viper.BindPFlag("client.local_port", rootCmd.Flags().Lookup("local-port"))
	viper.BindPFlag("client.listen_addr", rootCmd.Flags().Lookup("listen-addr"))
	viper.BindPFlag("client.remote_host", rootCmd.Flags().Lookup("remote-host"))
	viper.BindPFlag("client.remote_port", rootCmd.Flags().Lookup("remote-port"))
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
	viper.BindEnv("client.local_port", "LOCAL_PORT")
	viper.BindEnv("client.listen_addr", "LISTEN_ADDR")
	viper.BindEnv("client.remote_host", "REMOTE_HOST")
	viper.BindEnv("client.remote_port", "REMOTE_PORT")
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

	// Validate required configuration
	localPort := viper.GetInt("client.local_port")
	if localPort == 0 {
		return fmt.Errorf("local port is required")
	}

	remoteHost := viper.GetString("client.remote_host")
	remotePort := viper.GetInt("client.remote_port")
	if remoteHost == "" || remotePort == 0 {
		return fmt.Errorf("remote host and port are required")
	}

	// Initialize NATS transport
	transport := natsTransport.NewNATSTransport()

	// Connect to NATS
	natsConfig := &natsTransport.NATSConfig{
		URL:     viper.GetString("nats.url"),
		Timeout: 30 * time.Second,
	}

	ctx := context.Background()
	if err := transport.Connect(ctx, natsConfig); err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer transport.Close()

	// Start client
	listenAddr := viper.GetString("client.listen_addr")
	listenPort := viper.GetInt("client.local_port")
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenAddr, listenPort))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	logger.WithFields(logrus.Fields{
		"listen_addr": fmt.Sprintf("%s:%d", listenAddr, listenPort),
		"remote_host": remoteHost,
		"remote_port": remotePort,
		"nats_url":    natsConfig.URL,
	}).Info("Client started successfully")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				logger.WithError(err).Error("Failed to accept connection")
				continue
			}

			go handleClientConnection(ctx, conn, transport, logger, remoteHost, remotePort)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down client...")

	// Give connections time to close gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Close listener to stop accepting new connections
	listener.Close()

	// Wait for shutdown timeout or context cancellation
	<-shutdownCtx.Done()
	logger.Info("Client shutdown complete")

	return nil
}

// handleClientConnection handles a client connection by forwarding it through NATS
func handleClientConnection(ctx context.Context, clientConn net.Conn, natsTransport transport.Transport, logger *logrus.Entry, remoteHost string, remotePort int) {
	defer clientConn.Close()

	logger.WithField("client_addr", clientConn.RemoteAddr()).Info("New client connection")

	// Create a unique connection ID
	connID := fmt.Sprintf("conn_%d", time.Now().UnixNano())

	// Subscribe to responses for this connection
	responseSubject := fmt.Sprintf("proxy.response.%s", connID)
	msgChan, err := natsTransport.Subscribe(ctx, responseSubject)
	if err != nil {
		logger.WithError(err).Error("Failed to subscribe to response channel")
		return
	}

	// Create a channel to coordinate shutdown
	done := make(chan struct{})
	defer close(done)

	// Start reading from client and forwarding to NATS
	go func() {
		defer func() {
			// Send close message
			closeMsg := transport.NewMessage(connID, []byte("CLOSE"))
			natsTransport.Publish(ctx, "proxy.request", closeMsg)
		}()

		buffer := make([]byte, 32*1024) // 32KB buffer
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			default:
				// Set read timeout
				clientConn.SetReadDeadline(time.Now().Add(30 * time.Second))

				n, err := clientConn.Read(buffer)
				if err != nil {
					logger.WithError(err).Debug("Client connection read error")
					return
				}

				if n > 0 {
					// Create message with connection metadata
					msg := transport.NewMessageWithMetadata(connID, buffer[:n], map[string]string{
						"response_subject": responseSubject,
						"client_addr":      clientConn.RemoteAddr().String(),
						"remote_host":      remoteHost,
						"remote_port":      fmt.Sprintf("%d", remotePort),
					})

					// Send to NATS
					if err := natsTransport.Publish(ctx, "proxy.request", msg); err != nil {
						logger.WithError(err).Error("Failed to send message to NATS")
						return
					}

					logger.WithFields(logrus.Fields{
						"connection_id": connID,
						"bytes":         n,
					}).Debug("Forwarded data to NATS")
				}
			}
		}
	}()

	// Start reading responses from NATS and forwarding to client
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case msg := <-msgChan:
			if msg == nil {
				return
			}

			// Check if this is a close message
			if string(msg.Data) == "CLOSE" {
				logger.WithField("connection_id", connID).Info("Connection closed by server")
				return
			}

			// Write data to client
			clientConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if _, err := clientConn.Write(msg.Data); err != nil {
				logger.WithError(err).Error("Failed to write to client")
				return
			}

			logger.WithFields(logrus.Fields{
				"connection_id": connID,
				"bytes":         len(msg.Data),
			}).Debug("Forwarded data to client")
		}
	}
}
