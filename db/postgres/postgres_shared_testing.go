package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var TEST_TimeNow = time.Date(2000, time.January, 1, 12, 0, 0, 0, time.Local)
var TEST_RandomToken = "123"

func TESTONLY_SetupAndCleanup(t *testing.T) *PostgresDB {
	assert := assert.New(t)
	pgdb, err := NewPostgresDB(TESTONLY_Config)
	assert.NoError(err)
	pg := pgdb.(*PostgresDB)
	t.Cleanup(func() {
		sqlDB, err := pg.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	pg.db.Exec(`DROP TABLE IF EXISTS authentication_tokens CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS users CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS edge_edits CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS edges CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS node_edits CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS nodes CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS roles CASCADE`)
	pg.db.Exec(`DROP INDEX IF EXISTS idx_nodes_description_text_trgm;`)
	pg.db.Exec(`DROP EXTENSION IF EXISTS pg_trgm CASCADE;`)
	pgdb, err = NewPostgresDB(TESTONLY_Config)
	assert.NoError(err)
	pg = pgdb.(*PostgresDB)
	pg.newToken = func() string { return TEST_RandomToken }
	pg.timeNow = func() time.Time { return TEST_TimeNow }
	return pg
}
