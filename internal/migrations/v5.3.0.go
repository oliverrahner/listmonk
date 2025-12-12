package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V5_3_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	// Add incoming_email field to lists table for incoming mail processing.
	_, err := db.Exec(`
		ALTER TABLE lists ADD COLUMN IF NOT EXISTS incoming_email TEXT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_lists_incoming_email ON lists(incoming_email) WHERE incoming_email IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	return nil
}
