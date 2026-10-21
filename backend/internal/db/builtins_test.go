package db

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestSeedVersionNeverRestoresDeletedContent(t *testing.T) {
	database, mock, _ := sqlmock.New()
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO builtin_initializations").WithArgs(uint64(1), "skills-v1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := seedVersion(context.Background(), database, 1, "skills-v1", func(*sql.Tx) ([]uint64, error) { t.Fatal("reseeded"); return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestSeedVersionRollsBackFailedInitialization(t *testing.T) {
	database, mock, _ := sqlmock.New()
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO builtin_initializations").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	if err := seedVersion(context.Background(), database, 1, "skills-v1", func(*sql.Tx) ([]uint64, error) { return nil, errors.New("failed") }); err == nil {
		t.Fatal("failed seed committed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinsMigrationOnlyWidensExistingColumns(t *testing.T) {
	for _, length := range []int{32, 64} {
		database, mock, _ := sqlmock.New()
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS builtin_initializations").WillReturnResult(sqlmock.NewResult(0, 0))
		for _, col := range []struct{ table, name string }{{"tutor_skills", "name"}, {"tutor_messages", "skill_name"}} {
			mock.ExpectQuery("SELECT CHARACTER_MAXIMUM_LENGTH").WithArgs(col.table, col.name).WillReturnRows(sqlmock.NewRows([]string{"length"}).AddRow(length))
			if length < 64 {
				mock.ExpectExec("ALTER TABLE " + col.table + " MODIFY COLUMN " + col.name).WillReturnResult(sqlmock.NewResult(0, 0))
			}
		}
		if err := migrateBuiltins(context.Background(), database); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		database.Close()
	}
}
