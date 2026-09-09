// Package repository предоставляет реализацию слоя доступа к данным (Data Access Layer)
// для работы с базой данных PostgreSQL или хранения в памяти.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/eshadow1/gophkeeper/internal/config"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultDriver               = "postgres"
	defaultMaxIdleConnections   = 5
	defaultMaxOpenConnections   = 20
	defaultMinOpenConnections   = 5
	codePostgresDuplicateInsert = "23505"
	defaultConnMaxLifetime      = 1 * time.Minute
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrConflict          = errors.New("updated conflict")
)

type postgreSQLRepository struct {
	db   *sql.DB
	pool *pgxpool.Pool
}

// NewPostgreSQL создает и возвращает новый репозиторий для работы с PostgreSQL.
func NewPostgreSQL(cfg config.StorageConfig) (*postgreSQLRepository, error) {
	db, errOpen := sql.Open("pgx", cfg.PathDB)
	if errOpen != nil {
		return nil, fmt.Errorf("error create PostgreSQL DB: %w", errOpen)
	}

	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)

	if cfg.PathMigrations != "" {
		if errMigrate := runMigrationsWithDB(db, "file://"+cfg.PathMigrations); errMigrate != nil {
			return nil, fmt.Errorf("error migrate: %w", errMigrate)
		}
		loggers.Log.Info("Migrate successful")
	} else {
		loggers.Log.Info("Migrate disabled")
	}

	configPool, errParseConfig := pgxpool.ParseConfig(cfg.PathDB)
	if errParseConfig != nil {
		return nil, fmt.Errorf("error parse config: %w", errParseConfig)
	}

	configPool.MaxConns = defaultMaxOpenConnections
	configPool.MinConns = defaultMinOpenConnections
	configPool.MaxConnIdleTime = defaultConnMaxLifetime

	pool, errPool := pgxpool.NewWithConfig(context.Background(), configPool)
	if errPool != nil {
		return nil, fmt.Errorf("error parse config: %w", errPool)
	}

	return &postgreSQLRepository{
		db:   db,
		pool: pool,
	}, nil
}

// Close закрывает соединение с базой данных и освобождает все связанные ресурсы.
func (repo *postgreSQLRepository) Close() {
	if repo.pool != nil {
		repo.pool.Close()
	}
	if repo.db != nil {
		repo.db.Close()
	}
}

// CreateUser сохраняет нового пользователя в базе данных.
func (repo *postgreSQLRepository) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		INSERT INTO users (username, password_hash, created_at) 
		VALUES ($1, $2, $3) 
		RETURNING id, username, password_hash, created_at`

	err := repo.pool.QueryRow(ctx, query, user.Username, user.PasswordHash, user.CreatedAt).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == codePostgresDuplicateInsert {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// GetUser находит пользователя по имени.
func (repo *postgreSQLRepository) GetUser(ctx context.Context, username string) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, created_at 
		FROM users 
		WHERE username = $1;
	`
	user := &model.User{}
	err := repo.pool.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// CreateItem сохраняет новый элемент данных.
func (repo *postgreSQLRepository) CreateItem(ctx context.Context, item *model.ItemDB) (*model.ItemDB, error) {
	query := `
		INSERT INTO items (user_id, data_type, encrypted_data, meta_info, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6)
		
		RETURNING id, user_id, data_type, encrypted_data, meta_info, created_at, updated_at;
	`

	err := repo.pool.QueryRow(ctx, query, item.UserID, item.DataType, item.EncryptedData, item.MetaInfo, item.CreatedAt, item.UpdatedAt).
		Scan(&item.ID, &item.UserID, &item.DataType, &item.EncryptedData, &item.MetaInfo, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}
	return item, nil
}

// UpdateItem обновляет элемент данных.
func (repo *postgreSQLRepository) UpdateItem(ctx context.Context, item *model.ItemDB) error {
	query := `
		UPDATE items 
		SET data_type = $4, encrypted_data = $5, meta_info = $6, updated_at = NOW() 
		WHERE id = $1 and user_id = $2 and updated_at = $3;
	`
	tag, err := repo.pool.Exec(ctx, query, item.ID, item.UserID, item.UpdatedAt, item.DataType, item.EncryptedData, item.MetaInfo)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update item %s is not success: %w", item.ID, ErrConflict)
	}

	return nil
}

// DeleteItem удаляет элемент данных.
func (repo *postgreSQLRepository) DeleteItem(ctx context.Context, item *model.ItemDB) error {
	query := `DELETE FROM items WHERE id = $1 and user_id = $2;`

	_, err := repo.pool.Exec(ctx, query, item.ID, item.UserID)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// GetItemsByUserID возвращает все элементы данных для указанного пользователя.
func (repo *postgreSQLRepository) GetItemsByUserID(ctx context.Context, userID string) ([]*model.ItemDB, error) {
	query := `
		SELECT id, user_id, data_type, encrypted_data, meta_info, created_at, updated_at 
		FROM items 
		WHERE user_id = $1
		`

	rows, err := repo.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*model.ItemDB
	for rows.Next() {
		item := &model.ItemDB{}
		errScan := rows.Scan(&item.ID, &item.UserID, &item.DataType, &item.EncryptedData, &item.MetaInfo, &item.CreatedAt, &item.UpdatedAt)
		if errScan != nil {
			return nil, fmt.Errorf("failed to scan item: %w", errScan)
		}
		items = append(items, item)
	}
	return items, nil
}

func runMigrationsWithDB(db *sql.DB, migrationsPath string) error {
	driver, errInstance := postgres.WithInstance(db, &postgres.Config{})
	if errInstance != nil {
		return fmt.Errorf("failed to create postgres driver: %w", errInstance)
	}

	m, errDBInstance := migrate.NewWithDatabaseInstance(
		migrationsPath,
		defaultDriver,
		driver,
	)
	if errDBInstance != nil {
		return fmt.Errorf("failed to init migrate: %w", errDBInstance)
	}

	if errUpMigrate := m.Up(); errUpMigrate != nil && !errors.Is(errUpMigrate, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", errUpMigrate)
	}

	return nil
}
