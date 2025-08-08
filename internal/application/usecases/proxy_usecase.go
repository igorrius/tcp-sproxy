package usecases

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/repositories"
	"github.com/igorrius/tcp-sproxy/internal/domain/services"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
	"github.com/igorrius/tcp-sproxy/pkg/transport"
	"github.com/sirupsen/logrus"
)

// ProxyUseCase handles the proxy application logic
type ProxyUseCase struct {
	proxyService   *services.ProxyService
	connectionRepo repositories.ConnectionRepository
	transport      transport.Transport
	logger         *logrus.Entry
	remoteHost     string
	remotePort     int
	cleanupMutex   sync.Mutex // Prevents concurrent cleanup operations
}

// NewProxyUseCase creates a new proxy use case
func NewProxyUseCase(
	proxyService *services.ProxyService,
	connectionRepo repositories.ConnectionRepository,
	transport transport.Transport,
	remoteHost string,
	remotePort int,
) *ProxyUseCase {
	return &ProxyUseCase{
		proxyService:   proxyService,
		connectionRepo: connectionRepo,
		transport:      transport,
		remoteHost:     remoteHost,
		remotePort:     remotePort,
		logger:         logrus.WithField("usecase", "proxy"),
	}
}

// HandleNewConnection handles a new incoming connection
func (uc *ProxyUseCase) HandleNewConnection(ctx context.Context, clientConn net.Conn) error {
	uc.logger.WithFields(logrus.Fields{
		"client_addr": clientConn.RemoteAddr(),
		"remote_host": uc.remoteHost,
		"remote_port": uc.remotePort,
	}).Info("Handling new connection")

	return uc.proxyService.ProxyConnection(ctx, clientConn, uc.remoteHost, uc.remotePort)
}

// GetConnectionStats returns connection statistics
func (uc *ProxyUseCase) GetConnectionStats(ctx context.Context) (*services.ConnectionStats, error) {
	return uc.proxyService.GetConnectionStats(ctx)
}

// CloseConnection closes a specific connection
func (uc *ProxyUseCase) CloseConnection(ctx context.Context, connID valueobjects.ConnectionID) error {
	return uc.proxyService.CloseConnection(ctx, connID)
}

// CleanupOldConnections removes old closed connections
func (uc *ProxyUseCase) CleanupOldConnections(ctx context.Context, olderThan time.Duration) error {
	uc.cleanupMutex.Lock()
	defer uc.cleanupMutex.Unlock()
	return uc.connectionRepo.Cleanup(ctx, olderThan)
}

// StartCleanupScheduler starts a background cleanup scheduler
func (uc *ProxyUseCase) StartCleanupScheduler(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				uc.logger.Info("Cleanup scheduler stopped")
				return
			case <-ticker.C:
				if err := uc.CleanupOldConnections(ctx, 1*time.Hour); err != nil {
					uc.logger.WithError(err).Error("Failed to cleanup old connections")
				}
			}
		}
	}()
}
