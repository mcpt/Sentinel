package handlers

import "context"

type BackupHandler interface {
	Backup(ctx context.Context) (string, error)
	Name() string
}
