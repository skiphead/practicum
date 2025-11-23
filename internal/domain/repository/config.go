package repository

const (
	storageTableName   = "shorts_url"
	defaultBatchSize   = 100
	defaultExpiryYears = 1
)

type RepositoryConfig struct {
	TableName string
}

type RepositoryOption func(*storageRepository)
