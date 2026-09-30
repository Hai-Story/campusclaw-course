package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeletedSeedIsNotRestored(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dir := t.TempDir()
	item := seedMaterial{ClassName: "A 班", Title: "预置材料", StoredName: "seed-class-a.md", Content: "body"}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM deleted_seed_materials").WithArgs(item.StoredName).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if err := seedOneMaterial(context.Background(), database, dir, item); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, item.StoredName)); !os.IsNotExist(err) {
		t.Fatalf("deleted seed file restored: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
