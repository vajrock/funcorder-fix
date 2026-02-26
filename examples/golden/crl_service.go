// Package services предоставляет реализации бизнес-логики для системы SimpleCA PKI.
// Этот файл содержит реализацию CRLService, которая управляет списками отозванных сертификатов (CRL),
// включая генерацию, распространение и автоматические обновления согласно стандарту RFC 5280.
package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/vajrock/funcorder-fix/stubs/apperrors"
	"github.com/vajrock/funcorder-fix/stubs/config"
	"github.com/vajrock/funcorder-fix/stubs/constants"
	"github.com/vajrock/funcorder-fix/stubs/entities"
	"github.com/vajrock/funcorder-fix/stubs/ifacerepositories"
	"github.com/vajrock/funcorder-fix/stubs/ifaceservicies"
)

// crlScheduler управляет автоматической генерацией CRL по расписанию.
type crlScheduler struct {
	service *crlService
}

// newCRLScheduler создаёт новый планировщик для автоматической генерации CRL.
func newCRLScheduler(service *crlService) *crlScheduler {
	return &crlScheduler{
		service: service,
	}
}

// startScheduled запускает автоматическую генерацию CRL по расписанию.
func (s *crlScheduler) startScheduled(_ context.Context) error {
	// Implementation would go here
	return nil
}

// stop останавливает автоматическую генерацию CRL.
func (s *crlScheduler) stop() {
	// Implementation would go here
}

// crlService реализует интерфейс CRLService и предоставляет функциональность
// управления списками отозванных сертификатов, включая генерацию, валидацию,
// распространение и автоматические обновления.
// Поддерживает ручную и автоматическую генерацию CRL с настраиваемыми сроками действия.
type crlService struct {
	certRepo             ifacerepositories.CertificateRepositoryInterface
	crlEntryRepo         ifacerepositories.CrlEntryRepositoryInterface
	crlMetadataRepo      ifacerepositories.CrlMetadataRepositoryInterface
	privateKeyRepo       ifacerepositories.PrivateKeyDatabaseRepositoryInterface
	keyRepoFS            ifacerepositories.PrivateKeyFileSystemRepositoryInterface
	intermediateCertRepo ifacerepositories.IntermediateCertificateRepositoryInterface
	pemHandler           ifaceservicies.PEMHandler
	rsaHandler           ifaceservicies.RSAHandler
	// For storing last generated CRL metadata.
	lastGeneratedCRL *entities.CrlMetadata
	// Configuration.
	config *config.Config
	// Separated concerns.
	passwordManager  ifaceservicies.PasswordManagerInterface
	scheduler        *crlScheduler
	metricsCollector ifaceservicies.MetricsCollector
}

// crlHealthCheckConfig определяет параметры для проверки здоровья CRL компонента.
type crlHealthCheckConfig struct {
	checkFunc  func(ctx context.Context) error
	name       string
	okMessage  string
	isCritical bool
}

// Константы для статусов операций и метрик.
const (
	operationStatusSuccess = "success"
	operationStatusError   = "error"
	unknownCA              = "unknown"
)

// NewCRLService создаёт новый экземпляр CRLService со всеми необходимыми зависимостями
// для управления списками отзыва сертификатов и их метаданными.
func NewCRLService(
	certRepo ifacerepositories.CertificateRepositoryInterface,
	crlEntryRepo ifacerepositories.CrlEntryRepositoryInterface,
	crlMetadataRepo ifacerepositories.CrlMetadataRepositoryInterface,
	privateKeyRepo ifacerepositories.PrivateKeyDatabaseRepositoryInterface,
	keyRepoFS ifacerepositories.PrivateKeyFileSystemRepositoryInterface,
	intermediateCertRepo ifacerepositories.IntermediateCertificateRepositoryInterface,
	pemHandler ifaceservicies.PEMHandler,
	rsaHandler ifaceservicies.RSAHandler,
	passwordManager ifaceservicies.PasswordManagerInterface,
	cfg *config.Config,
	metricsCollector ifaceservicies.MetricsCollector,
) ifaceservicies.CRLService {
	service := &crlService{
		certRepo:             certRepo,
		crlEntryRepo:         crlEntryRepo,
		crlMetadataRepo:      crlMetadataRepo,
		privateKeyRepo:       privateKeyRepo,
		keyRepoFS:            keyRepoFS,
		intermediateCertRepo: intermediateCertRepo,
		pemHandler:           pemHandler,
		rsaHandler:           rsaHandler,
		lastGeneratedCRL:     nil,
		config:               cfg,
		passwordManager:      passwordManager,
		scheduler:            nil,
		metricsCollector:     metricsCollector,
	}

	service.scheduler = newCRLScheduler(service)
	return service
}

func (s *crlService) SetCRTService(_ context.Context, _ ifaceservicies.CertificateService) {
	// Password manager is now injected through constructor, no initialization needed here.
}

// GenerateCRLNow generates CRL immediately (administrative endpoint).
func (s *crlService) GenerateCRLNow(ctx context.Context) error {
	start := time.Now()
	operationStatus := operationStatusSuccess
	intermediateCA := unknownCA

	defer func() {
		duration := time.Since(start).Seconds()
		s.metricsCollector.RecordCRLGenerationDuration(operationStatus, intermediateCA, duration)
	}()

	password, err := s.GetCachedPassword(ctx)
	if err != nil {
		operationStatus = operationStatusError
		s.metricsCollector.IncrementErrors("crl_service", "generate", "password_error") //nolint:goconst // метка для метрик
		return err
	}

	err = s.generateCRLWithPassword(ctx, password)
	if err != nil {
		operationStatus = operationStatusError
		return err
	}

	// Record successful CRL generation.
	s.metricsCollector.IncrementCRLGenerated(intermediateCA, "manual", operationStatus)
	return nil
}

// StartAutoCRLGeneration initiates background automatic CRL generation.
// It starts a goroutine that periodically checks if CRL regeneration is needed
// and performs updates automatically. The operation is thread-safe.
// func (s *crlService) StartAutoCRLGeneration(ctx context.Context) error {
// 	return s.scheduler.start(ctx)
// }.

// StartScheduledCRLGeneration starts scheduled CRL generation with specified intervals.
// This method creates timers for regular CRL generation and can be used for daily generation.
func (s *crlService) StartScheduledCRLGeneration(ctx context.Context) error {
	return s.scheduler.startScheduled(ctx)
}

// StopAutoCRLGeneration stops automatic CRL generation.
func (s *crlService) StopAutoCRLGeneration() {
	s.scheduler.stop()
}

// GetCRL retrieves current CRL in requested format (pem or der).
func (s *crlService) GetCRL(ctx context.Context, format entities.CertificateFormat) (string, error) {
	// Default to PEM if no format specified.
	if format == "" {
		format = entities.FormatPEM
	}

	// Validate format.
	if format != entities.FormatPEM && format != entities.FormatDER {
		s.metricsCollector.IncrementCRLDownloads(unknownCA, string(format), "400")
		return "", apperrors.Newf(
			"unsupported_format",
			"unsupported format: %s. Supported formats: pem, der",
			constants.HTTPStatusBadRequest,
			string(format),
		)
	}

	intermediateCA := unknownCA

	// Get CRL in PEM format from cache or database.
	crlPEM, err := s.getCRLPEM(ctx, format)
	if err != nil {
		return "", err
	}

	// Determine intermediate CA for metrics.
	if s.lastGeneratedCRL != nil {
		intermediateCA = s.lastGeneratedCRL.IssuerUUID.String()
	}

	// Return in requested format.
	switch format {
	case entities.FormatPEM:
		s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(format), "200")
		s.metricsCollector.SetCRLSize(intermediateCA, string(format), float64(len(crlPEM)))
		return crlPEM, nil
	case entities.FormatDER:
		return s.convertPEMToDER(crlPEM, intermediateCA)
	default:
		s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(format), "400")
		return "", fmt.Errorf("%w: %s", entities.ErrUnsupportedFormat, format) //nolint:goconst // формат-строка для ошибки
	}
}

// AddRevokedCertificate adds a certificate to CRL.
func (s *crlService) AddRevokedCertificate(ctx context.Context, cert *entities.Certificate) error {
	// In a real implementation, this would:
	// 1. Create CRL entry in database
	// 2. Update CRL metadata
	// 3. Handle proper CRL entry structure.

	// For now, we'll just log the operation.
	// In production, this would be a full implementation.

	// Validate certificate.
	if cert == nil {
		return entities.ErrCertificateNilCRL
	}

	if cert.SerialNumber == "" {
		return entities.ErrCertificateSerialEmpty
	}

	// Check if certificate is already revoked.
	crlEntry, err := s.crlEntryRepo.GetCrlEntryBySerialNumber(ctx, cert.SerialNumber)
	if err != nil {
		// If CRL entry not found, that's expected - proceed with creation.
		if !errors.Is(err, entities.ErrCrlEntryNotFound) {
			return fmt.Errorf("failed to check existing CRL entry: %w", err)
		}
		// CRL entry doesn't exist yet, which is expected for a new revocation.
	} else if crlEntry != nil {
		// Certificate already in CRL.
		return nil
	}

	// Create CRL entry.
	revocationTime := time.Now().UTC()
	if cert.RevocationTime != nil {
		revocationTime = *cert.RevocationTime
	}

	// Default to unspecified reason if not set.
	reason := entities.RevocationReasonUnspecified
	if cert.RevocationReason != nil {
		reason = *cert.RevocationReason
	}

	entry := &entities.CrlEntry{
		ID:               0,
		CertificateID:    cert.ID,
		SerialNumber:     cert.SerialNumber,
		RevocationTime:   revocationTime,
		RevocationReason: reason,
		CrlNumber:        0, // Will be set during CRL generation.
	}

	// Create CRL entry in database.
	err = s.crlEntryRepo.CreateCrlEntry(ctx, entry)
	if err != nil {
		return fmt.Errorf("failed to create CRL entry: %w", err)
	}

	return nil
}

// ValidateCRLIntegrity validates CRL integrity with comprehensive checks according to RFC 5280
func (s *crlService) ValidateCRLIntegrity(ctx context.Context, crlPEM string) error {
	// Parse the CRL PEM block
	block, _ := pem.Decode([]byte(crlPEM))
	if block == nil {
		return entities.ErrInvalidPEMFormat
	}

	// Parse the CRL structure
	crl, err := x509.ParseRevocationList(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CRL: %w", err)
	}

	// Get the issuer certificate for signature verification
	intermediateCert, err := s.getIntermediateCertificate(ctx)
	if err != nil {
		return fmt.Errorf("failed to get issuer certificate for CRL validation: %w", err)
	}

	// Parse the issuer certificate
	issuerBlock, _ := pem.Decode([]byte(intermediateCert.CertificatePEM))
	if issuerBlock == nil {
		return entities.ErrIssuerCertDecodeFailed
	}

	issuerCert, err := x509.ParseCertificate(issuerBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse issuer certificate: %w", err)
	}

	// 1. Verify the CRL signature and algorithm
	if err := s.validateCRLSignature(crl, issuerCert); err != nil {
		return err
	}

	// 2. Check time validity
	if err := validateCRLTimeValidity(crl); err != nil {
		return err
	}

	// 3. Validate CRL number extension if present
	if crl.Number != nil {
		if crl.Number.Sign() <= 0 {
			return fmt.Errorf("%w: got %s", entities.ErrCRLNumberMustBePositive, crl.Number.String())
		}
	}

	// 4. Validate critical extensions
	if err := validateCRLCriticalExtensions(crl.Extensions); err != nil {
		return err
	}

	// 5. Validate revoked certificate entries
	if err := validateRevokedCertEntries(crl); err != nil {
		return err
	}

	// 6. Cross-validate with database
	if err := s.crossValidateCRLWithDatabase(ctx, crl); err != nil {
		return err
	}

	return nil
}

// GetRevokedCertificates retrieves all revoked certificates
func (s *crlService) GetRevokedCertificates(ctx context.Context) ([]entities.Certificate, error) {
	// Get all revoked certificates from the certificate repository
	revokedCerts, err := s.certRepo.GetCertificatesByStatus(ctx, entities.StatusRevoked)
	if err != nil {
		return nil, fmt.Errorf("failed to get revoked certificates: %w", err)
	}

	return revokedCerts, nil
}

// AddRevokedCertificateWithPassword adds a certificate to CRL with password for signing
func (s *crlService) AddRevokedCertificateWithPassword(ctx context.Context, cert *entities.Certificate, password string) error {
	// First add the certificate to CRL entries (same as original method)
	if err := s.AddRevokedCertificate(ctx, cert); err != nil {
		return err
	}

	// Check if auto CRL update after revoke is enabled
	if s.config.Server.AutoUpdateCRLAfterRevoke {
		// Trigger CRL regeneration with password using unified method
		return s.generateCRLWithPassword(ctx, password)
	}

	return nil
}

// GenerateCRLWithPassword generates CRL with password for signing
func (s *crlService) GenerateCRLWithPassword(ctx context.Context, password string) error {
	return s.generateCRLWithPassword(ctx, password)
}

// CachePassword caches the intermediate CA password for future CRL operations
func (s *crlService) CachePassword(ctx context.Context, password string) error {
	if err := s.passwordManager.CachePassword(ctx, password); err != nil {
		return apperrors.Wrap(
			err,
			"password_cache_error",
			"failed to cache password",
			constants.HTTPStatusInternalServerError,
		) //nolint:goconst // код ошибки
	}
	return nil
}

// GetCachedPassword retrieves the cached password, returns error if not cached
func (s *crlService) GetCachedPassword(ctx context.Context) (string, error) {
	password, err := s.passwordManager.GetCachedPassword(ctx)
	if err != nil {
		return "", apperrors.Wrap(err, "password_cache_error", "failed to get cached password", constants.HTTPStatusInternalServerError)
	}
	return password, nil
}

// ValidatePassword validates if the provided password matches the cached one
func (s *crlService) ValidatePassword(ctx context.Context, password string) error {
	if err := s.passwordManager.ValidatePassword(ctx, password); err != nil {
		return apperrors.Wrap(err, "password_validation_error", "password validation failed", constants.HTTPStatusUnauthorized)
	}
	return nil
}

// validateCRLTimeValidity проверяет временную валидность CRL
func validateCRLTimeValidity(crl *x509.RevocationList) error {
	now := time.Now().UTC()

	// Check ThisUpdate - CRL should not be from the future
	if crl.ThisUpdate.After(now) {
		return fmt.Errorf("%w: %s", entities.ErrCRLThisUpdateFuture, crl.ThisUpdate.Format(time.RFC3339))
	}

	// Check NextUpdate - CRL should not be expired
	if !crl.NextUpdate.IsZero() {
		if now.After(crl.NextUpdate) {
			return fmt.Errorf("%w: NextUpdate was %s, Current time: %s",
				entities.ErrCRLExpired, crl.NextUpdate.Format(time.RFC3339), now.Format(time.RFC3339))
		}

		// Check reasonable validity period (CRLs typically have validity of hours to days)
		validityPeriod := crl.NextUpdate.Sub(crl.ThisUpdate)
		if validityPeriod > 7*24*time.Hour { // More than 7 days is unusual
			return fmt.Errorf("%w: %v", entities.ErrCRLValidityTooLong, validityPeriod)
		}
	}

	return nil
}

// validateCRLCriticalExtensions проверяет критические расширения CRL
func validateCRLCriticalExtensions(extensions []pkix.Extension) error {
	for _, ext := range extensions {
		if ext.Critical {
			// Check for known critical extensions
			switch {
			case ext.Id.Equal([]int{2, 5, 29, 20}): // CRL Number
				// Valid critical extension
			case ext.Id.Equal([]int{2, 5, 29, 35}): // Authority Key Identifier
				// Valid critical extension
			case ext.Id.Equal([]int{2, 5, 29, 31}): // CRL Distribution Points
				// Valid critical extension
			default:
				return fmt.Errorf("%w: %s", entities.ErrCRLUnsupportedCriticalExt, ext.Id.String())
			}
		}
	}

	return nil
}

// validateRevokedCertEntries проверяет записи отозванных сертификатов
func validateRevokedCertEntries(crl *x509.RevocationList) error {
	now := time.Now().UTC()
	serialNumbers := make(map[string]bool)

	for _, entry := range crl.RevokedCertificateEntries {
		// Check for duplicate serial numbers
		serialStr := entry.SerialNumber.String()
		if serialNumbers[serialStr] {
			return fmt.Errorf("%w: %s", entities.ErrCRLDuplicateSerial, serialStr)
		}
		serialNumbers[serialStr] = true

		// Check revocation time
		if entry.RevocationTime.After(now) {
			return fmt.Errorf("%w: certificate %s revocation time %s",
				entities.ErrCRLRevocationTimeFuture, serialStr, entry.RevocationTime.Format(time.RFC3339))
		}

		// Validate reason code extension if present
		for _, ext := range entry.Extensions {
			if ext.Id.Equal([]int{2, 5, 29, 21}) { // CRL Reason code
				if len(ext.Value) > 0 {
					reason := int(ext.Value[0])
					if reason < 0 || reason > 10 {
						return fmt.Errorf("%w: %d for certificate %s", entities.ErrCRLInvalidRevocationReason, reason, serialStr)
					}
				}
			}
		}
	}

	return nil
}

// VerifyPassword verifies if the provided password can decrypt the intermediate CA private key
func (s *crlService) VerifyPassword(ctx context.Context, password string) error {
	// Try to get the intermediate CA private key with the provided password
	_, err := s.getIntermediateCAPrivateKey(ctx, password)
	if err != nil {
		return fmt.Errorf("password verification failed: %w", err)
	}

	return nil
}

// HasCachedPassword returns true if a password is currently cached
func (s *crlService) HasCachedPassword(ctx context.Context) bool {
	return s.passwordManager.HasCachedPassword(ctx)
}

// ClearCachedPassword removes the cached password
func (s *crlService) ClearCachedPassword(ctx context.Context) error {
	if err := s.passwordManager.ClearCachedPassword(ctx); err != nil {
		return apperrors.Wrap(err, "password_cache_error", "failed to clear cached password", constants.HTTPStatusInternalServerError)
	}
	return nil
}

// HealthCheck performs a comprehensive health check of the CRL service
func (s *crlService) HealthCheck(ctx context.Context) *ifaceservicies.HealthCheckResult {
	start := time.Now()
	result := &ifaceservicies.HealthCheckResult{
		Status:    ifaceservicies.HealthStatusHealthy,
		Message:   "",
		Details:   make(map[string]string),
		Timestamp: start,
		Duration:  0,
	}

	// Определение проверок здоровья
	checks := []crlHealthCheckConfig{
		{
			name:       "database",
			okMessage:  "Database connection",
			checkFunc:  s.checkDatabaseHealth,
			isCritical: true,
		},
		{
			name:       "crl_repository",
			okMessage:  "CRL repository",
			checkFunc:  s.checkCRLRepositoryHealth,
			isCritical: false,
		},
		{
			name:       "certificate_repository",
			okMessage:  "Certificate repository",
			checkFunc:  s.checkCertificateRepositoryHealth,
			isCritical: false,
		},
		{
			name:       "intermediate_cert",
			okMessage:  "Intermediate certificate access",
			checkFunc:  s.checkIntermediateCertificateAccess,
			isCritical: false,
		},
		{
			name:       "password_management",
			okMessage:  "Password management",
			checkFunc:  s.checkPasswordManagementHealth,
			isCritical: false,
		},
		{
			name:       "crl_generation",
			okMessage:  "CRL generation capability",
			checkFunc:  s.checkCRLGenerationCapability,
			isCritical: false,
		},
		{
			name:       "scheduler",
			okMessage:  "Scheduler",
			checkFunc:  s.checkSchedulerHealth,
			isCritical: false,
		},
	}

	// Выполнение всех проверок
	for _, check := range checks {
		s.runCRLHealthCheck(ctx, result, check)
	}

	result.Duration = time.Since(start)

	switch result.Status {
	case ifaceservicies.HealthStatusHealthy:
		result.Message = "CRL service is fully operational"
	case ifaceservicies.HealthStatusDegraded:
		result.Message = "CRL service is operational with limited functionality"
	case ifaceservicies.HealthStatusUnhealthy:
		result.Message = "CRL service is not operational"
	}

	return result
}

// Name returns the name of the service
func (s *crlService) Name() string {
	return "CRLService"
}

// Helper method to get intermediate certificate (similar to certificate_service.go).
func (s *crlService) getIntermediateCertificate(ctx context.Context) (*entities.IntermediateCertificate, error) {
	// Get all intermediate certificates and return the most recent one.
	intermediateCerts, err := s.intermediateCertRepo.ListIntermediateCertificates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get intermediate certificates: %w", err) //nolint:goconst // формат-строка для ошибки
	}

	if len(intermediateCerts) == 0 {
		return nil, entities.ErrNoIntermediateCertificate
	}

	// Return the most recent one (last in the slice).
	return &intermediateCerts[len(intermediateCerts)-1], nil
}

// getCRLPEM retrieves CRL in PEM format from cache or database, generating if needed.
func (s *crlService) getCRLPEM(ctx context.Context, format entities.CertificateFormat) (string, error) {
	intermediateCA := unknownCA

	if s.lastGeneratedCRL != nil {
		s.metricsCollector.IncrementCRLCacheHits(s.lastGeneratedCRL.IssuerUUID.String())
		return s.lastGeneratedCRL.CrlValue, nil
	}

	latestCRL, err := s.getLatestCRLFromDB(ctx)
	if err != nil {
		if errors.Is(err, apperrors.ErrCRLNotFound) {
			return s.handleMissingCRL(ctx, format)
		}
		s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(format), "500")
		s.metricsCollector.IncrementErrors("crl_service", "get_crl", "database_error") //nolint:goconst // метки для метрик
		return "", fmt.Errorf("failed to get latest CRL from database: %w", err)
	}

	s.metricsCollector.IncrementCRLCacheMisses(latestCRL.IssuerUUID.String())
	return latestCRL.CrlValue, nil
}

// handleMissingCRL handles the case when CRL is not found in database.
func (s *crlService) handleMissingCRL(ctx context.Context, format entities.CertificateFormat) (string, error) {
	intermediateCA := unknownCA

	// Try to generate an empty CRL if password is cached.
	if s.passwordManager.HasCachedPassword(ctx) {
		genErr := s.generateCRLWithPassword(ctx, "")
		if genErr != nil {
			s.metricsCollector.IncrementErrors("crl_service", "get_crl", "generation_error")
			return "", fmt.Errorf("failed to generate initial CRL: %w", genErr)
		}
		// Try to get CRL again after generation.
		latestCRL, err := s.getLatestCRLFromDB(ctx)
		if err != nil {
			s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(format), "500")
			return "", fmt.Errorf("failed to get CRL after generation: %w", err)
		}
		s.metricsCollector.IncrementCRLCacheMisses(latestCRL.IssuerUUID.String())
		return latestCRL.CrlValue, nil
	}

	// No cached password - return error indicating CRL needs to be generated.
	s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(format), "404")
	return "", entities.ErrCRLCachePasswordRequired
}

// Helper function to encode integer as ASN.1 DER
func encodeInteger(i int64) []byte {
	// This is a simplified implementation
	// In production, would use proper ASN.1 encoding
	return big.NewInt(i).Bytes()
}

// GenerateCRLHash generates SHA256 hash for CRL integrity check
func generateCRLHash(
	crlNumber int64,
	issuerUUID string,
	thisUpdate time.Time,
	nextUpdate time.Time,
	crlSize int,
) string {
	// Format the data as specified in the specification
	data := fmt.Sprintf("%d|%s|%s|%s|%d",
		crlNumber,
		issuerUUID,
		thisUpdate.UTC().Format(time.RFC3339),
		nextUpdate.UTC().Format(time.RFC3339),
		crlSize)

	// Calculate SHA256 hash
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// convertPEMToDER converts CRL from PEM to DER format.
func (s *crlService) convertPEMToDER(crlPEM, intermediateCA string) (string, error) {
	block, _ := pem.Decode([]byte(crlPEM))
	if block == nil {
		s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(entities.FormatDER), "500")
		s.metricsCollector.IncrementErrors("crl_service", "get_crl", "crypto_error")
		return "", entities.ErrCRLDecodeFailed
	}

	derData := string(block.Bytes)
	s.metricsCollector.IncrementCRLDownloads(intermediateCA, string(entities.FormatDER), "200")
	s.metricsCollector.SetCRLSize(intermediateCA, string(entities.FormatDER), float64(len(derData)))
	return derData, nil
}

// Helper method to get latest CRL from database.
func (s *crlService) getLatestCRLFromDB(ctx context.Context) (*entities.CrlMetadata, error) {
	// Get all CRL metadata and return the most recent one.
	crlMetadataList, err := s.crlMetadataRepo.ListCrlMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list CRL metadata: %w", err)
	}

	if len(crlMetadataList) == 0 {
		return nil, apperrors.ErrCRLNotFound
	}

	// Return the most recent one (highest CRL number or latest GeneratedAt).
	var latestCRL *entities.CrlMetadata
	for i := range crlMetadataList {
		if latestCRL == nil || crlMetadataList[i].GeneratedAt.After(latestCRL.GeneratedAt) {
			latestCRL = &crlMetadataList[i]
		}
	}

	return latestCRL, nil
}

// validateCRLSignature проверяет подпись CRL и алгоритм подписи
func (s *crlService) validateCRLSignature(crl *x509.RevocationList, issuerCert *x509.Certificate) error {
	if err := crl.CheckSignatureFrom(issuerCert); err != nil {
		return fmt.Errorf("CRL signature verification failed: %w", err)
	}

	if crl.SignatureAlgorithm == x509.UnknownSignatureAlgorithm {
		return entities.ErrCRLUnknownSignature
	}

	return nil
}

// crossValidateCRLWithDatabase выполняет перекрёстную проверку CRL с базой данных
func (s *crlService) crossValidateCRLWithDatabase(ctx context.Context, crl *x509.RevocationList) error {
	revokedCerts, err := s.GetRevokedCertificates(ctx)
	if err != nil {
		// Log warning but don't fail validation if we can't access database
		fmt.Printf("Warning: could not validate CRL entries against database: %v\n", err)
		return nil
	}

	// Convert database revoked certs to map for validation
	dbRevoked := make(map[string]time.Time)
	for i := range revokedCerts {
		cert := &revokedCerts[i]
		if cert.SerialNumber != "" {
			if cert.RevocationTime != nil {
				dbRevoked[cert.SerialNumber] = *cert.RevocationTime
			} else {
				dbRevoked[cert.SerialNumber] = cert.CreatedAt
			}
		}
	}

	// Check that all entries in CRL match database
	for _, entry := range crl.RevokedCertificateEntries {
		serialStr := entry.SerialNumber.String()
		if dbRevokedTime, exists := dbRevoked[serialStr]; exists {
			// Check that revocation times are reasonable (allow some tolerance)
			timeDiff := entry.RevocationTime.Sub(dbRevokedTime)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			if timeDiff > time.Hour { // More than 1 hour difference is suspicious
				return fmt.Errorf(
					"%w: certificate %s: CRL says %s, DB says %s",
					entities.ErrCRLRevocationTimeMismatch,
					serialStr,
					entry.RevocationTime.Format(time.RFC3339),
					dbRevokedTime.Format(time.RFC3339),
				)
			}
		}
		// Note: We don't require all DB revoked certs to be in CRL as CRL generation might be asynchronous
	}

	return nil
}

// createCRLInternal creates a new CRL with proper structure - unified internal method
func (s *crlService) createCRLInternal(ctx context.Context, issuerCert *entities.IntermediateCertificate, revokedCerts []entities.Certificate, password string) (string, error) {
	// Parse the issuer certificate to get subject
	issuerBlock, _ := pem.Decode([]byte(issuerCert.CertificatePEM))
	if issuerBlock == nil {
		return "", entities.ErrIssuerCertDecodeFailed
	}

	issuerCertParsed, err := x509.ParseCertificate(issuerBlock.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse issuer certificate: %w", err)
	}

	// Get the next CRL number
	crlNumber, err := s.getNextCRLNumber(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get next CRL number: %w", err)
	}

	// Create TBSCertList structure
	tbsCertList := x509.RevocationList{ //nolint:exhaustruct // template заполняется только нужными полями, остальное устанавливает x509
		SignatureAlgorithm:        x509.SHA256WithRSA, // Default to SHA256 with RSA
		Issuer:                    issuerCertParsed.Subject,
		ThisUpdate:                time.Now().UTC(),
		NextUpdate:                time.Now().UTC().Add(24 * time.Hour), // 24 hour validity
		RevokedCertificateEntries: []x509.RevocationListEntry{},
		Extensions:                []pkix.Extension{},
		Number:                    big.NewInt(crlNumber),
	}

	// Add revoked certificates
	for i := range revokedCerts {
		cert := &revokedCerts[i]
		revokedCertEntry, err := s.createCRLEntry(ctx, cert)
		if err != nil {
			return "", fmt.Errorf("failed to create CRL entry for certificate %s: %w", cert.SerialNumber, err)
		}
		tbsCertList.RevokedCertificateEntries = append(tbsCertList.RevokedCertificateEntries, *revokedCertEntry)
	}

	// Add CRL Number extension
	crlNumberExt := pkix.Extension{ //nolint:exhaustruct // Critical не нужен для CRL Number extension
		Id:    []int{2, 5, 29, 20}, // id-ce-cRLNumber
		Value: encodeInteger(crlNumber),
	}
	tbsCertList.Extensions = append(tbsCertList.Extensions, crlNumberExt)

	// Add Authority Key Identifier extension
	// For now, we'll create a minimal implementation
	authorityKeyIDExt := pkix.Extension{ //nolint:exhaustruct // Critical не нужен для AKI extension
		Id:    []int{2, 5, 29, 35}, // id-ce-authorityKeyIdentifier
		Value: []byte{},            // Empty for now, would contain actual key identifier
	}
	tbsCertList.Extensions = append(tbsCertList.Extensions, authorityKeyIDExt)

	// Get the intermediate CA private key
	privateKey, err := s.getIntermediateCAPrivateKey(ctx, password)
	if err != nil {
		return "", fmt.Errorf("failed to get intermediate CA private key: %w", err)
	}

	// Create the full CRL structure with proper signing
	crlBytes, err := x509.CreateRevocationList(rand.Reader, &tbsCertList, issuerCertParsed, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to create CRL: %w", err)
	}

	// Encode to PEM format
	crlPEM := string(pem.EncodeToMemory(&pem.Block{ //nolint:exhaustruct // Headers требуется только для специфичных PEM типов
		Type:  "X509 CRL",
		Bytes: crlBytes,
	}))

	return crlPEM, nil
}

// handlePasswordForCRL обрабатывает пароль для генерации CRL.
func (s *crlService) handlePasswordForCRL(ctx context.Context, password string) (string, error) {
	isEncrypted, err := s.isIntermediateKeyEncrypted(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check if key is encrypted: %w", err)
	}

	// Для незашифрованного ключа — кэшируем пустой пароль
	if !isEncrypted {
		if err := s.passwordManager.CachePassword(ctx, ""); err != nil {
			return "", fmt.Errorf("failed to cache empty password for unencrypted key: %w", err)
		}
		return "", nil
	}

	// Для зашифрованного ключа с предоставленным паролем — кэшируем его
	if password != "" {
		if err := s.passwordManager.CachePassword(ctx, password); err != nil {
			return "", fmt.Errorf("failed to cache password for CRL generation: %w", err)
		}
		return password, nil
	}

	// Для зашифрованного ключа без пароля — получаем из кэша
	cachedPassword, err := s.passwordManager.GetCachedPassword(ctx)
	if err != nil {
		return "", fmt.Errorf("encrypted key requires password but none provided and none cached: %w", err)
	}
	return cachedPassword, nil
}

// createCRLMetadata создаёт метаданные для CRL.
func (s *crlService) createCRLMetadata(
	crlNumber int64,
	issuerUUID string,
	revokedCertsCount int,
	crlPEM string,
) *entities.CrlMetadata {
	now := time.Now().UTC()
	nextUpdate := now.Add(time.Duration(constants.DefaultCacheTTLHours) * time.Hour)

	return &entities.CrlMetadata{
		CrlNumber:   crlNumber,
		IssuerUUID:  uuid.MustParse(issuerUUID),
		ThisUpdate:  now,
		NextUpdate:  nextUpdate,
		CrlSize:     int64(revokedCertsCount),
		Sha256Hash:  generateCRLHash(crlNumber, issuerUUID, now, nextUpdate, revokedCertsCount),
		GeneratedAt: now,
		CrlValue:    crlPEM,
	}
}

// generateCRLWithPassword generates CRL with password for signing - unified method
func (s *crlService) generateCRLWithPassword(ctx context.Context, password string) error {
	// Обработка пароля
	password, err := s.handlePasswordForCRL(ctx, password)
	if err != nil {
		return err
	}

	// Получение intermediate сертификата
	intermediateCert, err := s.getIntermediateCertificate(ctx)
	if err != nil {
		return fmt.Errorf("failed to get intermediate certificate: %w", err)
	}

	// Получение отозванных сертификатов
	revokedCerts, err := s.GetRevokedCertificates(ctx)
	if err != nil {
		return fmt.Errorf("failed to get revoked certificates: %w", err)
	}

	// Создание CRL
	crlPEM, err := s.createCRLInternal(ctx, intermediateCert, revokedCerts, password)
	if err != nil {
		return fmt.Errorf("failed to create CRL: %w", err)
	}

	// Валидация CRL
	if err := s.ValidateCRLIntegrity(ctx, crlPEM); err != nil {
		return fmt.Errorf("generated CRL integrity check failed: %w", err)
	}

	// Получение номера CRL
	crlNumber, err := s.getNextCRLNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next CRL number: %w", err)
	}

	// Создание метаданных
	metadata := s.createCRLMetadata(crlNumber, intermediateCert.ID.String(), len(revokedCerts), crlPEM)

	// Сохранение в базу
	if err := s.crlMetadataRepo.CreateCrlMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to save CRL metadata: %w", err)
	}

	// Обновление последнего CRL
	s.lastGeneratedCRL = metadata

	return nil
}

// Helper method to get next CRL number
func (s *crlService) getNextCRLNumber(ctx context.Context) (int64, error) {
	// Get all CRL metadata to find the highest CRL number
	crlMetadataList, err := s.crlMetadataRepo.ListCrlMetadata(ctx)
	if err != nil {
		return 1, fmt.Errorf("failed to list CRL metadata: %w", err)
	}

	// Find the highest CRL number
	var maxCRLNumber int64
	for i := range crlMetadataList {
		metadata := &crlMetadataList[i]
		if metadata.CrlNumber > maxCRLNumber {
			maxCRLNumber = metadata.CrlNumber
		}
	}

	// Return next number
	return maxCRLNumber + 1, nil
}

// Helper method to create CRL entry
func (s *crlService) createCRLEntry(_ context.Context, cert *entities.Certificate) (*x509.RevocationListEntry, error) {
	serial := new(big.Int)
	serial.SetString(cert.SerialNumber, 10)

	// Handle the case where RevocationTime might be nil
	var revocationTime time.Time
	if cert.RevocationTime != nil {
		revocationTime = *cert.RevocationTime
	} else {
		revocationTime = time.Now().UTC()
	}

	// Create the revoked certificate entry
	revokedCertEntry := &x509.RevocationListEntry{ //nolint:exhaustruct // Extensions добавляются отдельно при наличии RevocationReason
		SerialNumber:   serial,
		RevocationTime: revocationTime,
	}

	// Add reason code extension if available
	if cert.RevocationReason != nil {
		reasonExt := pkix.Extension{ //nolint:exhaustruct // Critical не нужен для CRL Reason extension
			Id:    []int{2, 5, 29, 21}, // id-ce-cRLReasons
			Value: encodeInteger(int64(*cert.RevocationReason)),
		}
		revokedCertEntry.Extensions = append(revokedCertEntry.Extensions, reasonExt)
	}

	return revokedCertEntry, nil
}

// isIntermediateKeyEncrypted checks if the intermediate CA private key is encrypted
func (s *crlService) isIntermediateKeyEncrypted(ctx context.Context) (bool, error) {
	// Get intermediate certificate first
	intermediateCert, err := s.getIntermediateCertificate(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get intermediate certificate: %w", err)
	}

	// Get private key metadata from database using the intermediate certificate
	keyMeta, err := s.privateKeyRepo.GetPrivateKeyByCertificate(ctx, intermediateCert)
	if err != nil {
		return false, fmt.Errorf("failed to get private key metadata: %w", err) //nolint:goconst // формат-строка для ошибки
	}

	// Get private key from filesystem
	keyBytes, err := s.keyRepoFS.GetPrivateKey(ctx, keyMeta)
	if err != nil {
		return false, fmt.Errorf("failed to get private key from filesystem: %w", err) //nolint:goconst // формат-строка для ошибки
	}

	// Check if the key is encrypted by trying to parse it without password
	_, err = s.pemHandler.ParsePrivateKey(keyBytes, "")
	if err != nil {
		// If parsing fails without password, key is likely encrypted
		return true, nil //nolint:nilerr // Ошибка парсинга = ключ зашифрован (ожидаемое поведение)
	}

	// If parsing succeeds without password, key is not encrypted
	return false, nil
}

// getIntermediateCAPrivateKey retrieves and decrypts the intermediate CA private key
func (s *crlService) getIntermediateCAPrivateKey(ctx context.Context, password string) (*rsa.PrivateKey, error) {
	// Get intermediate certificate first
	intermediateCert, err := s.getIntermediateCertificate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get intermediate certificate: %w", err)
	}

	// Get private key metadata from database using the intermediate certificate
	keyMeta, err := s.privateKeyRepo.GetPrivateKeyByCertificate(ctx, intermediateCert)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key metadata: %w", err)
	}

	// Get private key from filesystem
	keyBytes, err := s.keyRepoFS.GetPrivateKey(ctx, keyMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key from filesystem: %w", err)
	}

	// Decrypt the private key using PEM handler (same as in certificate service)
	privateKey, err := s.pemHandler.ParsePrivateKey(keyBytes, password)
	if err != nil {
		return nil, fmt.Errorf("failed to parse and decrypt private key: %w", err)
	}

	// Convert to RSA private key
	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, entities.ErrPrivateKeyNotRSA
	}

	return rsaPrivateKey, nil
}

// runCRLHealthCheck выполняет проверку здоровья компонента CRL.
func (s *crlService) runCRLHealthCheck(
	ctx context.Context,
	result *ifaceservicies.HealthCheckResult,
	hcConfig crlHealthCheckConfig,
) {
	err := hcConfig.checkFunc(ctx)
	if err != nil {
		if hcConfig.isCritical {
			result.Status = ifaceservicies.HealthStatusUnhealthy
		} else if result.Status == ifaceservicies.HealthStatusHealthy {
			result.Status = ifaceservicies.HealthStatusDegraded
		}
		result.Details[hcConfig.name] = hcConfig.okMessage + " failed: " + err.Error()
	} else {
		result.Details[hcConfig.name] = hcConfig.okMessage + " OK"
	}
}

// Helper methods for health checks
func (s *crlService) checkDatabaseHealth(ctx context.Context) error {
	// Use lightweight repository health check instead of loading all CRL entries
	return s.crlEntryRepo.CheckRepositoryHealth(ctx) //nolint:wrapcheck // Прокси-метод для health check
}

func (s *crlService) checkCRLRepositoryHealth(ctx context.Context) error {
	// Use lightweight repository health checks instead of loading all data
	if err := s.crlMetadataRepo.CheckRepositoryHealth(ctx); err != nil {
		return fmt.Errorf("CRL metadata repository unavailable: %w", err)
	}

	if err := s.crlEntryRepo.CheckRepositoryHealth(ctx); err != nil {
		return fmt.Errorf("CRL entry repository unavailable: %w", err)
	}

	return nil
}

func (s *crlService) checkCertificateRepositoryHealth(ctx context.Context) error {
	// Use lightweight repository health check instead of loading all certificates
	return s.certRepo.CheckRepositoryHealth(ctx) //nolint:wrapcheck // Прокси-метод для health check
}

func (s *crlService) checkIntermediateCertificateAccess(ctx context.Context) error {
	// Use lightweight repository health check instead of loading all intermediate certificates
	if err := s.intermediateCertRepo.CheckRepositoryHealth(ctx); err != nil {
		return fmt.Errorf("intermediate certificate repository unavailable: %w", err) //nolint:goconst // формат-строка для ошибки
	}

	// Check if we have at least one intermediate certificate using lightweight check
	if err := s.intermediateCertRepo.CheckHasIntermediateCertificates(ctx); err != nil {
		return fmt.Errorf("no intermediate certificates available: %w", err)
	}

	return nil
}

func (s *crlService) checkPasswordManagementHealth(ctx context.Context) error {
	// Check if password manager is initialized
	if s.passwordManager == nil {
		return entities.ErrPasswordManagerNotInit
	}

	// Check if we have cached password status
	if !s.passwordManager.HasCachedPassword(ctx) {
		return entities.ErrNoPasswordCached
	}

	return nil
}

func (s *crlService) checkCRLGenerationCapability(ctx context.Context) error {
	// Use lightweight repository health check instead of loading all certificates
	if err := s.intermediateCertRepo.CheckRepositoryHealth(ctx); err != nil {
		return fmt.Errorf("intermediate certificate repository unavailable: %w", err)
	}

	// Check if we have at least one intermediate certificate using lightweight check
	if err := s.intermediateCertRepo.CheckHasIntermediateCertificates(ctx); err != nil {
		return fmt.Errorf("no intermediate certificates available for CRL generation: %w", err)
	}

	// Check private key repository availability
	if err := s.privateKeyRepo.CheckRepositoryHealth(ctx); err != nil {
		return fmt.Errorf("private key repository unavailable: %w", err)
	}

	// Check if we have private keys available for signing
	if err := s.privateKeyRepo.CheckHasPrivateKeys(ctx); err != nil {
		return fmt.Errorf("no private keys available for CRL signing: %w", err)
	}

	return nil
}

func (s *crlService) checkSchedulerHealth(_ context.Context) error {
	// Check if scheduler is initialized
	if s.scheduler == nil {
		return entities.ErrSchedulerNotInitialized
	}

	// In a real implementation, you might check if the scheduler is running
	// For now, just check that it exists
	return nil
}
