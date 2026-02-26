package entities

import (
	"time"

	"github.com/google/uuid"
)

// CertificateFormat определяет формат сертификата.
type CertificateFormat string

const (
	// FormatPEM - PEM формат.
	FormatPEM CertificateFormat = "pem"
	// FormatDER - DER формат.
	FormatDER CertificateFormat = "der"
)

// Статусы сертификатов.
const (
	StatusRevoked = "revoked"
)

// Причины отзыва сертификатов.
const (
	RevocationReasonUnspecified = 0
)

// Ошибки пакета entities.
var (
	ErrNoIntermediateCertificate  = error(nil)
	ErrInvalidPEMFormat           = error(nil)
	ErrIssuerCertDecodeFailed     = error(nil)
	ErrCRLNumberMustBePositive    = error(nil)
	ErrCRLExpired                 = error(nil)
	ErrCRLValidityTooLong         = error(nil)
	ErrCRLUnsupportedCriticalExt  = error(nil)
	ErrCRLDuplicateSerial         = error(nil)
	ErrCRLRevocationTimeFuture    = error(nil)
	ErrCRLInvalidRevocationReason = error(nil)
	ErrCRLRevocationTimeMismatch  = error(nil)
	ErrCertificateNilCRL          = error(nil)
	ErrCertificateSerialEmpty     = error(nil)
	ErrCrlEntryNotFound           = error(nil)
	ErrUnsupportedFormat          = error(nil)
	ErrCRLDecodeFailed            = error(nil)
	ErrCRLCachePasswordRequired   = error(nil)
	ErrCRLUnknownSignature        = error(nil)
	ErrPrivateKeyNotRSA           = error(nil)
	ErrPasswordManagerNotInit     = error(nil)
	ErrNoPasswordCached           = error(nil)
	ErrSchedulerNotInitialized    = error(nil)
	ErrCRLThisUpdateFuture        = error(nil)
)

// CrlMetadata представляет метаданные CRL.
type CrlMetadata struct {
	CrlNumber   int64
	IssuerUUID  uuid.UUID
	ThisUpdate  time.Time
	NextUpdate  time.Time
	CrlSize     int64
	Sha256Hash  string
	GeneratedAt time.Time
	CrlValue    string
}

// IntermediateCertificate представляет промежуточный сертификат.
type IntermediateCertificate struct {
	ID             uuid.UUID
	CertificatePEM string
}

// Certificate представляет сертификат.
type Certificate struct {
	ID               int64
	SerialNumber     string
	RevocationTime   *time.Time
	RevocationReason *int
	CreatedAt        time.Time
}

// CrlEntry представляет запись в CRL.
type CrlEntry struct {
	ID               int64
	CertificateID    int64
	SerialNumber     string
	RevocationTime   time.Time
	RevocationReason int
	CrlNumber        int64
}
