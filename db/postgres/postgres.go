package postgres

import (
	"context"
	"fmt"

	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/db/arangodb"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Node struct {
	gorm.Model
	Description db.Text `gorm:"type:jsonb;default:'[]';not null"`
}

func NewPostgresDB(conf db.Config) (db.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: fmt.Sprintf("host=%s user=learngraph password=example dbname=learngraph port=5432 sslmode=disable", conf.PGHost),
		// Note: we must disable caching when running migrations, while clients are active,
		// see https://github.com/jackc/pgx/wiki/Automatic-Prepared-Statement-Caching#automatic-prepared-statement-caching
		//PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	pg := &PostgresDB{
		db: db,
	}
	return pg.init()
}

type PostgresDB struct {
	db *gorm.DB
}

func (pg *PostgresDB) init() (db.DB, error) {
	return pg, pg.db.AutoMigrate(&Node{})
}

func (pg *PostgresDB) Graph(ctx context.Context) (*model.Graph, error) {
	return nil, nil
}
func (pg *PostgresDB) CreateNode(ctx context.Context, user db.User, description *model.Text) (string, error) {
	node := Node{Description: arangodb.ConvertToDBText(description)}
	tx := pg.db.Create(&node)
	return fmt.Sprint(node.ID), tx.Error
}
func (pg *PostgresDB) CreateEdge(ctx context.Context, user db.User, from, to string, weight float64) (string, error) {
	return "", nil
}
func (pg *PostgresDB) EditNode(ctx context.Context, user db.User, nodeID string, description *model.Text) error {
	return nil
}
func (pg *PostgresDB) AddEdgeWeightVote(ctx context.Context, user db.User, edgeID string, weight float64) error {
	return nil
}
func (pg *PostgresDB) CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error) {
	return nil, nil
}
func (pg *PostgresDB) Login(ctx context.Context, auth model.LoginAuthentication) (*model.LoginResult, error) {
	return nil, nil
}
func (pg *PostgresDB) DeleteAccount(ctx context.Context) error {
	return nil
}
func (pg *PostgresDB) Logout(ctx context.Context) error {
	return nil
}
func (pg *PostgresDB) IsUserAuthenticated(ctx context.Context) (bool, *db.User, error) {
	return false, nil, nil
}
