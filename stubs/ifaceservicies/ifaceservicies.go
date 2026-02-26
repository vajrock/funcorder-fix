package ifaceservicies

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/vajrock/funcorder-fix/stubs/entities"
)

// PEMHandler определяет интерфейс для работы с PEM форматом.
type PEMHandler interface {
	ParsePrivateKey(data []byte, password string) (any, error)
}

// RSAHandler определяет интерфейс для работы с RSA.
type RSAHandler interface{}

// PasswordManagerInterface определяет интерфейс менеджера паролей.
type PasswordManagerInterface interface {
	CachePassword(ctx context.Context, password string) error
	GetCachedPassword(ctx context.Context) (string, error)
	ValidatePassword(ctx context.Context, password string) error
	HasCachedPassword(ctx context.Context) bool
	ClearCachedPassword(ctx context.Context) error
}

// MetricsCollector определяет интерфейс сбора метрик.
type MetricsCollector interface {
	RecordCRLGenerationDuration(status, intermediateCA string, duration float64)
	IncrementErrors(service, operation, errorType string)
	IncrementCRLGenerated(intermediateCA, triggerType, status string)
	IncrementCRLDownloads(intermediateCA, format, statusCode string)
	SetCRLSize(intermediateCA, format string, size float64)
	IncrementCRLCacheHits(intermediateCA string)
	IncrementCRLCacheMisses(intermediateCA string)
}

// CRLService определяет интерфейс сервиса CRL.
type CRLService interface {
	GenerateCRLNow(ctx context.Context) error
	StartScheduledCRLGeneration(ctx context.Context) error
	StopAutoCRLGeneration()
	GetCRL(ctx context.Context, format entities.CertificateFormat) (string, error)
	AddRevokedCertificate(ctx context.Context, cert *entities.Certificate) error
	HealthCheck(ctx context.Context) *HealthCheckResult
	Name() string
}

// CertificateService определяет интерфейс сервиса сертификатов.
type CertificateService interface{}

// HealthStatus представляет статус здоровья.
type HealthStatus string

const (
	// HealthStatusHealthy - сервис работает нормально.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded - сервис работает с ограничениями.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy - сервис не работает.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheckResult представляет результат проверки здоровья.
type HealthCheckResult struct {
	Status    HealthStatus
	Message   string
	Details   map[string]string
	Timestamp time.Time
	Duration  time.Duration
}

// PrivateKey представляет приватный ключ.
type PrivateKey = *rsa.PrivateKey
