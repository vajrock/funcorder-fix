package ifacerepositories

import (
	"context"

	"github.com/vajrock/funcorder-fix/stubs/entities"
)

// CertificateRepositoryInterface определяет интерфейс репозитория сертификатов.
type CertificateRepositoryInterface interface {
	GetCertificatesByStatus(ctx context.Context, status string) ([]entities.Certificate, error)
	CheckRepositoryHealth(ctx context.Context) error
}

// CrlEntryRepositoryInterface определяет интерфейс репозитория записей CRL.
type CrlEntryRepositoryInterface interface {
	GetCrlEntryBySerialNumber(ctx context.Context, serialNumber string) (*entities.CrlEntry, error)
	CreateCrlEntry(ctx context.Context, entry *entities.CrlEntry) error
	CheckRepositoryHealth(ctx context.Context) error
}

// CrlMetadataRepositoryInterface определяет интерфейс репозитория метаданных CRL.
type CrlMetadataRepositoryInterface interface {
	ListCrlMetadata(ctx context.Context) ([]entities.CrlMetadata, error)
	CreateCrlMetadata(ctx context.Context, metadata *entities.CrlMetadata) error
	CheckRepositoryHealth(ctx context.Context) error
}

// PrivateKeyDatabaseRepositoryInterface определяет интерфейс репозитория приватных ключей в БД.
type PrivateKeyDatabaseRepositoryInterface interface {
	GetPrivateKeyByCertificate(ctx context.Context, cert *entities.IntermediateCertificate) (any, error)
	CheckRepositoryHealth(ctx context.Context) error
	CheckHasPrivateKeys(ctx context.Context) error
}

// PrivateKeyFileSystemRepositoryInterface определяет интерфейс репозитория приватных ключей в ФС.
type PrivateKeyFileSystemRepositoryInterface interface {
	GetPrivateKey(ctx context.Context, keyMeta any) ([]byte, error)
}

// IntermediateCertificateRepositoryInterface определяет интерфейс репозитория промежуточных сертификатов.
type IntermediateCertificateRepositoryInterface interface {
	ListIntermediateCertificates(ctx context.Context) ([]entities.IntermediateCertificate, error)
	CheckRepositoryHealth(ctx context.Context) error
	CheckHasIntermediateCertificates(ctx context.Context) error
}
