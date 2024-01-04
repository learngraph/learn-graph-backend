package postgres

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/db/arangodb"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Node struct {
	gorm.Model
	Description db.Text `gorm:"type:jsonb;default:'{}';not null"`
}
type NodeEdit struct {
	gorm.Model
	NodeID  uint
	Node    Node `gorm:"constraint:OnDelete:CASCADE;not null"`
	UserID  uint
	User    User            `gorm:"constraint:OnDelete:CASCADE;not null"`
	Type    db.NodeEditType `gorm:"type:text;not null"`
	NewNode db.Text         `gorm:"type:jsonb;default:'{}';not null"`
}
type Edge struct {
	gorm.Model
	FromID uint `gorm:"index:noDuplicateEdges,unique;"`
	ToID   uint `gorm:"index:noDuplicateEdges,unique;"`
	From   Node `gorm:"constraint:OnDelete:CASCADE;not null"`
	To     Node `gorm:"constraint:OnDelete:CASCADE;not null"`
	Weight float64
}
type EdgeEdit struct {
	gorm.Model
	EdgeID uint
	Edge   Edge `gorm:"constraint:OnDelete:CASCADE;not null"`
	UserID uint
	User   User            `gorm:"constraint:OnDelete:CASCADE;not null"`
	Type   db.EdgeEditType `gorm:"type:text;not null"`
	Weight float64
}
type User struct {
	gorm.Model
	Username     string `gorm:"not null"`
	PasswordHash string `gorm:"not null"`
	EMail        string `gorm:"not null"`
	//Tokens       []AuthenticationToken `json:"authenticationtokens,omitempty"`
	//Roles        []RoleType            `json:"roles,omitempty"`
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
	return pg, pg.db.AutoMigrate(&Node{}, &Edge{}, &NodeEdit{}, &EdgeEdit{}, &User{})
}

func (pg *PostgresDB) Graph(ctx context.Context) (*model.Graph, error) {
	return nil, nil
}
func (pg *PostgresDB) CreateNode(ctx context.Context, user db.User, description *model.Text) (string, error) {
	node := Node{Description: arangodb.ConvertToDBText(description)}
	err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&node).Error; err != nil {
			return err
		}
		nodeedit := NodeEdit{
			NodeID:  node.ID,
			UserID:  atoi(user.Key),
			Type:    db.NodeEditTypeCreate,
			NewNode: node.Description,
		}
		if err := tx.Create(&nodeedit).Error; err != nil {
			return err
		}
		return nil
	})
	return itoa(node.ID), err
}
func (pg *PostgresDB) CreateEdge(ctx context.Context, user db.User, from, to string, weight float64) (string, error) {
	edge := Edge{
		FromID: atoi(from),
		ToID:   atoi(to),
		Weight: weight,
	}
	err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&edge).Error; err != nil {
			return err
		}
		edgeedit := EdgeEdit{
			EdgeID: edge.ID,
			UserID: atoi(user.Key),
			Type:   db.EdgeEditTypeCreate,
			Weight: weight,
		}
		if err := tx.Create(&edgeedit).Error; err != nil {
			return err
		}
		return nil
	})
	return itoa(edge.ID), err
}
func (pg *PostgresDB) EditNode(ctx context.Context, user db.User, nodeID string, description *model.Text) error {
	return pg.db.Transaction(func(tx *gorm.DB) error {
		node := Node{Model: gorm.Model{ID: atoi(nodeID)}}
		if err := tx.First(&node).Error; err != nil {
			return err
		}
		node.Description = mergeText(node.Description, arangodb.ConvertToDBText(description))
		if err := tx.Save(&node).Error; err != nil {
			return err
		}
		nodeedit := NodeEdit{
			NodeID:  atoi(nodeID),
			UserID:  atoi(user.Key),
			Type:    db.NodeEditTypeEdit,
			NewNode: node.Description,
		}
		if err := tx.Create(&nodeedit).Error; err != nil {
			return err
		}
		return nil
	})
}
func (pg *PostgresDB) AddEdgeWeightVote(ctx context.Context, user db.User, edgeID string, weight float64) error {
	return pg.db.Transaction(func(tx *gorm.DB) error {
		{
			// TODO(skep): should move aggregation to separate module/application
			edge := Edge{Model: gorm.Model{ID: atoi(edgeID)}}
			if err := tx.First(&edge).Error; err != nil {
				return err
			}
			edits := []EdgeEdit{}
			if err := tx.Where(&EdgeEdit{EdgeID: edge.ID}).Find(&edits).Error; err != nil {
				return err
			}
			sum := db.Sum(edits, func(edit EdgeEdit) float64 { return edit.Weight })
			averageWeight := (sum + weight) / float64(len(edits)+1)
			edge.Weight = averageWeight
			if err := tx.Save(&edge).Error; err != nil {
				return err
			}
		}
		edgeedit := EdgeEdit{
			EdgeID: atoi(edgeID),
			UserID: atoi(user.Key),
			Type:   db.EdgeEditTypeVote,
			Weight: weight,
		}
		if err := tx.Create(&edgeedit).Error; err != nil {
			return err
		}
		return nil
	})
}
func (pg *PostgresDB) CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error) {
	user := User{
		Username: username,
		EMail:    email,
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create password hash for user '%v'", user)
	}
	user.PasswordHash = string(hash)
	if err := pg.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &model.CreateUserResult{Login: &model.LoginResult{
		Success:  true,
		Token:    "123",
		UserID:   itoa(user.ID),
		UserName: user.Username,
	}}, nil
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
func (pg *PostgresDB) DeleteNode(ctx context.Context, user db.User, ID string) error {
	return nil
}
func (pg *PostgresDB) DeleteEdge(ctx context.Context, user db.User, ID string) error {
	return nil
}
