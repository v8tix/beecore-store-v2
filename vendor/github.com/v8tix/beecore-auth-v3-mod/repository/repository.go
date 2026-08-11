package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db          *pgxpool.Pool
	KmsKey      string
	KmsClient   KeySigner
	pubKeyCache *publicKeyCache
}

func NewRepository(db *pgxpool.Pool, kmsKey string, kmsClient KeySigner) Repository {
	return Repository{
		db:          db,
		KmsClient:   kmsClient,
		KmsKey:      kmsKey,
		pubKeyCache: &publicKeyCache{},
	}
}
