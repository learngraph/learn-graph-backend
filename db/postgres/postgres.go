package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/db/arangodb"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	AUTHENTICATION_TOKEN_EXPIRY = 12 * 30 * 24 * time.Hour // ~ 1 year
	AUTH_TOKEN_LENGTH           = 64                       // bytes
)

type Node struct {
	gorm.Model
	Description db.Text `gorm:"type:jsonb;default:'{}';not null"`
	Resources   db.Text `gorm:"type:jsonb"`
}
type NodeEdit struct {
	gorm.Model
	NodeID         uint
	Node           Node `gorm:"constraint:OnDelete:CASCADE;not null"`
	UserID         uint
	User           User            `gorm:"constraint:OnDelete:SET DEFAULT;not null"` // TODO(skep): deleted users should default to a "deleted-user" inserted here
	Type           db.NodeEditType `gorm:"type:text;not null"`
	NewDescription db.Text         `gorm:"type:jsonb;default:'{}';not null"`
	NewResources   db.Text         `gorm:"type:jsonb"`
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
	User   User            `gorm:"constraint:OnDelete:SET DEFAULT;not null"`
	Type   db.EdgeEditType `gorm:"type:text;not null"`
	Weight float64
}
type User struct {
	gorm.Model
	Username     string                `gorm:"not null;index:noDuplicateUsernames,unique;"`
	PasswordHash string                `gorm:"not null"`
	EMail        string                `gorm:"not null;index:noDuplicateEMails,unique;"`
	Tokens       []AuthenticationToken `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	//Roles        []RoleType            `json:"roles,omitempty"`
}
type AuthenticationToken struct {
	gorm.Model
	Token  string
	Expiry time.Time
	UserID uint
}

func makeStringToken() string {
	rnd := make([]byte, AUTH_TOKEN_LENGTH)
	n, err := rand.Read(rnd)
	if err != nil || n != AUTH_TOKEN_LENGTH {
		panic("not enough entropy")
	}
	dst := make([]byte, AUTH_TOKEN_LENGTH*4/3+0x10)
	base64.StdEncoding.Encode(dst, rnd)
	return string(dst)
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
		db:       db,
		timeNow:  time.Now,
		newToken: makeStringToken,
	}
	return pg.init()
}

type PostgresDB struct {
	db       *gorm.DB
	timeNow  func() time.Time
	newToken func() string
}

func (pg *PostgresDB) init() (db.DB, error) {
	return pg, pg.db.AutoMigrate(&Node{}, &Edge{}, &NodeEdit{}, &EdgeEdit{}, &AuthenticationToken{}, &User{})
}

func (pg *PostgresDB) Graph(ctx context.Context) (*model.Graph, error) {
	var (
		nodes []Node
		edges []Edge
	)
	err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Find(&nodes).Error; err != nil {
			return err
		}
		if err := tx.Find(&edges).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read graph")
	}
	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).Graph(nodes, edges), nil
}

func (pg *PostgresDB) Node(ctx context.Context, ID string) (*model.Node, error) {
	node := Node{}
	if err := pg.db.First(&node).Error; err != nil {
		return nil, err
	}
	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).Node(node), nil
}

func (pg *PostgresDB) CreateNode(ctx context.Context, user db.User, description, resources *model.Text) (string, error) {
	node := Node{Description: arangodb.ConvertToDBText(description), Resources: arangodb.ConvertToDBText(resources)}
	err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&node).Error; err != nil {
			return err
		}
		nodeedit := NodeEdit{
			NodeID:         node.ID,
			UserID:         atoi(user.Key),
			Type:           db.NodeEditTypeCreate,
			NewDescription: node.Description,
			NewResources:   node.Resources,
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
func (pg *PostgresDB) EditNode(ctx context.Context, user db.User, nodeID string, description, resources *model.Text) error {
	return pg.db.Transaction(func(tx *gorm.DB) error {
		node := Node{Model: gorm.Model{ID: atoi(nodeID)}}
		if err := tx.First(&node).Error; err != nil {
			return err
		}
		node.Description = mergeText(node.Description, arangodb.ConvertToDBText(description))
		node.Resources = mergeText(node.Resources, arangodb.ConvertToDBText(resources))
		if err := tx.Save(&node).Error; err != nil {
			return err
		}
		nodeedit := NodeEdit{
			NodeID:         atoi(nodeID),
			UserID:         atoi(user.Key),
			Type:           db.NodeEditTypeEdit,
			NewDescription: node.Description,
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
	if res := arangodb.VerifyUserInput(ctx, db.User{Username: username, EMail: email}, password); res != nil {
		return res, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create password hash for user '%v', '%v'", username, email)
	}
	user := User{
		Username:     username,
		EMail:        email,
		PasswordHash: string(hash),
		Tokens: []AuthenticationToken{
			{
				Token:  pg.newToken(),
				Expiry: pg.timeNow().Add(AUTHENTICATION_TOKEN_EXPIRY),
			},
		},
	}
	if err := pg.db.Create(&user).Error; err != nil {
		return nil, errors.Wrap(err, "failed to create user")
	}
	return &model.CreateUserResult{Login: &model.LoginResult{
		Success:  true,
		Token:    user.Tokens[0].Token,
		UserID:   itoa(user.ID),
		UserName: user.Username,
	}}, nil
}

func (pg *PostgresDB) Login(ctx context.Context, auth model.LoginAuthentication) (*model.LoginResult, error) {
	user := User{EMail: auth.Email}
	token := AuthenticationToken{Token: pg.newToken(), Expiry: pg.timeNow().Add(AUTHENTICATION_TOKEN_EXPIRY)}
	passwordMissmatch := false
	err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&user).First(&user).Error; err != nil {
			return err
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(auth.Password)); err != nil {
			passwordMissmatch = true
			return nil
		}
		token.UserID = user.ID
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user")
	}
	if passwordMissmatch {
		msg := "Password missmatch"
		return &model.LoginResult{
			Success: false,
			Message: &msg,
		}, nil
	}
	return &model.LoginResult{
		Success:  true,
		Token:    token.Token,
		UserID:   itoa(user.ID),
		UserName: user.Username,
	}, nil
}

func makeIsValidTokenFn(pg *PostgresDB, token string) func(at AuthenticationToken) bool {
	return func(at AuthenticationToken) bool {
		return at.Token == token && at.Expiry.After(pg.timeNow())
	}
}

func (pg *PostgresDB) IsUserAuthenticated(ctx context.Context) (bool, *db.User, error) {
	token := middleware.CtxGetAuthentication(ctx)
	user := User{Model: gorm.Model{ID: atoi(middleware.CtxGetUserID(ctx))}}
	if err := pg.db.Where(&user).Preload("Tokens").First(&user).Error; err != nil {
		return false, nil, errors.Wrapf(err, "failed to get user with token='%s', id='%v'", token, user.ID)
	}
	if db.FindFirst(user.Tokens, makeIsValidTokenFn(pg, token)) == nil {
		return false, nil, nil
	}
	dbUser := db.User{Document: db.Document{Key: itoa(user.ID)}, Username: user.Username, EMail: user.EMail}
	return true, &dbUser, nil
}

func (pg *PostgresDB) DeleteNode(ctx context.Context, user db.User, ID string) error {
	if err := pg.db.Transaction(func(tx *gorm.DB) error {
		var (
			edits int64
			edges int64
		)
		if err := tx.Model(&NodeEdit{}).Where("user_id != ?", ID).Count(&edits).Error; err != nil {
			return err
		}
		if edits >= 1 {
			return errors.New("node has edits from other users, won't delete")
		}
		if err := tx.Model(&Edge{}).Where("from_id = ? OR to_id = ?", ID, ID).Count(&edges).Error; err != nil {
			return err
		}
		if edges >= 1 {
			return errors.New("cannot delete node with edges, remove edges first")
		}
		if err := tx.Delete(&Node{Model: gorm.Model{ID: atoi(ID)}}).Error; err != nil {
			return err
		}
		return tx.Where("node_id = ?", ID).Delete(&NodeEdit{}).Error
	}); err != nil {
		return errors.Wrap(err, "transaction failed")
	}
	return nil
}

func (pg *PostgresDB) DeleteEdge(ctx context.Context, user db.User, ID string) error {
	if err := pg.db.Transaction(func(tx *gorm.DB) error {
		var (
			edits int64
		)
		if err := tx.Model(&EdgeEdit{}).Where("user_id != ?", ID).Count(&edits).Error; err != nil {
			return err
		}
		if edits >= 1 {
			return errors.New("node has edits from other users, won't delete")
		}
		if err := tx.Delete(&Edge{Model: gorm.Model{ID: atoi(ID)}}).Error; err != nil {
			return err
		}
		return tx.Where("edge_id = ?", ID).Delete(&EdgeEdit{}).Error
	}); err != nil {
		return errors.Wrap(err, "transaction failed")
	}
	return nil
}

func (pg *PostgresDB) Logout(ctx context.Context) error {
	token := middleware.CtxGetAuthentication(ctx)
	user := User{Model: gorm.Model{ID: atoi(middleware.CtxGetUserID(ctx))}}
	if err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&user).Preload("Tokens").First(&user).Error; err != nil {
			return err
		}
		idx := db.FindFirstIndex(user.Tokens, makeIsValidTokenFn(pg, token))
		if idx == -1 {
			return errors.New("no token found")
		}
		user.Tokens = db.DeleteAt(user.Tokens, idx)
		return tx.Save(&user).Error
	}); err != nil {
		return errors.Wrap(err, "transaction failed")
	}
	return nil
}

func (pg *PostgresDB) DeleteAccount(ctx context.Context) error {
	return ErrTODONotYetImplemented
}

var ErrTODONotYetImplemented = errors.New("TODO: implement") // TODO: remove once, migration is done
