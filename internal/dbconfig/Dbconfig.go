package dbconfig

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	// Registration
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func ConnectDB(dsn string) (*sql.DB, error) {
	// Format: "username:password@tcp(127.0.0.1:3306)/dbname"
	var err error
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 10 max open connections for learning
	db.SetMaxOpenConns(25)

	// 5 idle connections for better performance
	db.SetMaxIdleConns(25)

	// The expiry of the connections
	db.SetConnMaxLifetime(time.Minute * 5)

	// Verify the connection is actually alive
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database unreachable: %w", err)
	}

	return db, nil
}

// It automatically handles Commit if the function returns nil, or Rollback if it returns an error.
func WithTransaction(db *sql.DB, ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Defer a rollback in case of a panic or unexpected exit
	defer tx.Rollback()

	// Execute the repository logic
	if err := fn(tx); err != nil {
		return err
	}

	// If no error, commit the transaction
	return tx.Commit()
}

// For point-in-time transaction
func WithTransactionReadOnly(db *sql.DB, ctx context.Context, fn func(*sql.Tx) error) error {
	// Start the Snapshotted Transaction
	opts := &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Safety Net

	// Execute the repository logic
	if err := fn(tx); err != nil {
		return err
	}

	// If no error, commit the transaction
	return tx.Commit()
}

// Internal helper used by everything
func withTransactionInternal(db *sql.DB, ctx context.Context, opts *sql.TxOptions, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func RunMigrations(db *sql.DB) {
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		log.Fatal("[CRITICAL] Could not create migration driver:", err)
	}

	// path/to/migrations is relative to main.go
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"mysql",
		driver,
	)
	if err != nil {
		log.Fatal("[CRITICAL] Migration setup failed:", err)
	}

	// Apply all available migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal("[CRITICAL] Migration failed to run:", err)
	}

	log.Println("Database migrations applied successfully!")
}
