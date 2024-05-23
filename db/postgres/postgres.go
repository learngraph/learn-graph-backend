package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	AUTHENTICATION_TOKEN_EXPIRY = 12 * 30 * 24 * time.Hour // ~ 1 year
	AUTH_TOKEN_LENGTH           = 64                       // bytes
	MIN_PASSWORD_LENGTH         = 10
	MIN_USERNAME_LENGTH         = 4
)

var TESTONLY_Config = db.Config{PGHost: "localhost"}

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
	Username     string                `gorm:"not null;unique;"`
	PasswordHash string                `gorm:"not null"`
	EMail        string                `gorm:"not null;unique;"`
	Tokens       []AuthenticationToken `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Roles        []Role                `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
type AuthenticationToken struct {
	gorm.Model
	Token  string
	Expiry time.Time
	UserID uint
}
type Role struct {
	gorm.Model
	UserID uint        `gorm:"index:noDuplicateRolesPerUser,unique"`
	Role   db.RoleType `gorm:"index:noDuplicateRolesPerUser,unique;type:text;not null"`
}

func makeStringToken() string {
	rnd := make([]byte, AUTH_TOKEN_LENGTH)
	n, err := rand.Read(rnd)
	if err != nil || n != AUTH_TOKEN_LENGTH {
		panic("not enough entropy")
	}
	dst := make([]byte, AUTH_TOKEN_LENGTH*4/3+0x10)
	base64.StdEncoding.Encode(dst, rnd)
	return strings.Trim(string(dst), "\x00")
}

func NewPostgresDB(conf db.Config) (db.DB, error) {
	pgConfig := postgres.Config{
		DSN: fmt.Sprintf("host=%s user=learngraph password=%s dbname=learngraph port=5432 sslmode=disable", conf.PGHost, conf.PGPassword),
		// Note: we must disable caching when running migrations, while clients are active,
		// see https://github.com/jackc/pgx/wiki/Automatic-Prepared-Statement-Caching#automatic-prepared-statement-caching
		//PreferSimpleProtocol: true,
	}
	db, err := gorm.Open(postgres.New(pgConfig), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrapf(err, "authentication with DSN: '%v' failed", pgConfig.DSN)
	}
	pg := &PostgresDB{
		db:       db,
		timeNow:  time.Now,
		newToken: makeStringToken,
	}
	return pg.init()
}

// implements db.DB
type PostgresDB struct {
	db       *gorm.DB
	timeNow  func() time.Time
	newToken func() string
}

func (pg *PostgresDB) init() (db.DB, error) {
	return pg, pg.db.AutoMigrate(
		&Node{}, &Edge{}, &NodeEdit{}, &EdgeEdit{}, &AuthenticationToken{}, &User{}, &Role{},
	)
}

func removeArangoPrefix(s string) string {
	parts := strings.Split(s, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return s
}

// ReplaceAllDataWith DROPPS ALL DATA currently in the database, and replaces
// it with the passed data.
func (pg *PostgresDB) ReplaceAllDataWith(ctx context.Context, data db.AllData) error {
	err := pg.db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range []string{
			`DROP TABLE IF EXISTS authentication_tokens CASCADE`,
			`DROP TABLE IF EXISTS users CASCADE`,
			`DROP TABLE IF EXISTS edge_edits CASCADE`,
			`DROP TABLE IF EXISTS edges CASCADE`,
			`DROP TABLE IF EXISTS node_edits CASCADE`,
			`DROP TABLE IF EXISTS nodes CASCADE`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to drop old data")
	}
	if _, err := pg.init(); err != nil {
		return errors.Wrap(err, "failed to initialize new data")
	}
	users := []User{}
	for _, user := range data.Users {
		tokens := []AuthenticationToken{}
		for _, token := range user.Tokens {
			tokenExpiry := time.UnixMilli(token.Expiry)
			tokenExpiry = tokenExpiry.UTC()
			tokens = append(tokens, AuthenticationToken{
				Token:  token.Token,
				Expiry: tokenExpiry,
			})
		}
		users = append(users, User{
			Model:        gorm.Model{ID: atoi(user.Key)},
			Username:     user.Username,
			PasswordHash: user.PasswordHash,
			EMail:        user.EMail,
			Tokens:       tokens,
		})
	}
	nodes := []Node{}
	for _, node := range data.Nodes {
		nodes = append(nodes, Node{
			Model:       gorm.Model{ID: atoi(node.Key)},
			Description: node.Description,
			Resources:   node.Resources,
		})
	}
	edges := []Edge{}
	for _, edge := range data.Edges {
		edges = append(edges, Edge{
			Model:  gorm.Model{ID: atoi(edge.Key)},
			FromID: atoi(removeArangoPrefix(edge.From)),
			ToID:   atoi(removeArangoPrefix(edge.To)),
			Weight: edge.Weight,
		})
	}
	nodeedits := []NodeEdit{}
	for _, nodeedit := range data.NodeEdits {
		nodeedits = append(nodeedits, NodeEdit{
			NodeID:         atoi(nodeedit.Node),
			UserID:         atoi(nodeedit.User),
			Type:           nodeedit.Type,
			NewDescription: nodeedit.NewNode.Description,
			NewResources:   nodeedit.NewNode.Resources,
		})
	}
	edgeedits := []EdgeEdit{}
	for _, edgeedit := range data.EdgeEdits {
		edgeedits = append(edgeedits, EdgeEdit{
			EdgeID: atoi(edgeedit.Edge),
			UserID: atoi(edgeedit.User),
			Type:   edgeedit.Type,
			Weight: edgeedit.Weight,
		})
	}
	// only add those slices, that contain data
	//all := []interface{}{users, nodes, edges, nodeedits, edgeedits}
	allThatContainData := []interface{}{}
	if len(users) > 0 {
		allThatContainData = append(allThatContainData, &users)
	}
	if len(nodes) > 0 {
		allThatContainData = append(allThatContainData, &nodes)
	}
	if len(edges) > 0 {
		allThatContainData = append(allThatContainData, &edges)
	}
	if len(nodeedits) > 0 {
		allThatContainData = append(allThatContainData, &nodeedits)
	}
	if len(edgeedits) > 0 {
		allThatContainData = append(allThatContainData, &edgeedits)
	}
	return pg.db.Transaction(func(tx *gorm.DB) error {
		for _, thing := range allThatContainData {
			if err := tx.Create(thing).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
	graph := NewConvertToModel(lang).Graph(nodes, edges)
	return graph, nil
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
	node := Node{Description: db.ConvertToDBText(description), Resources: db.ConvertToDBText(resources)}
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
		node.Description = mergeText(node.Description, db.ConvertToDBText(description))
		node.Resources = mergeText(node.Resources, db.ConvertToDBText(resources))
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

// VerifyUserInput returns a CreateUserResult with an error message on
// *invalid* user input, on valid user input nil is returned.
func VerifyUserInput(ctx context.Context, user db.User, password string) *model.CreateUserResult {
	if len(password) < MIN_PASSWORD_LENGTH {
		msg := fmt.Sprintf("Password must be at least length %d, the provided one has only %d characters.", MIN_PASSWORD_LENGTH, len(password))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	if len(user.Username) < MIN_USERNAME_LENGTH {
		msg := fmt.Sprintf("Username must be at least length %d, the provided one has only %d characters.", MIN_USERNAME_LENGTH, len(user.Username))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	if _, err := mail.ParseAddress(user.EMail); err != nil {
		msg := fmt.Sprintf("Invalid EMail: '%s'", user.EMail)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	return nil
}

func (pg *PostgresDB) CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error) {
	if res := VerifyUserInput(ctx, db.User{Username: username, EMail: email}, password); res != nil {
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
		msg := fmt.Sprintf("%v", errors.Wrap(err, "failed to get user"))
		return &model.LoginResult{
			Success: false,
			Message: &msg,
		}, nil
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
		return false, nil, nil // no such user
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
		if err := tx.Model(&NodeEdit{}).Where("node_id = ? AND user_id != ?", ID, user.Key).Count(&edits).Error; err != nil {
			return err
		}
		isAdmin, err := isUserAdmin(tx, user.Key)
		if err != nil {
			return err
		}
		if edits >= 1 && !isAdmin {
			return errors.New("node has edits from other users, won't delete")
		}
		if err := tx.Model(&Edge{}).
			Joins("JOIN edge_edits ON edges.id = edge_edits.edge_id").
			Where("(edges.from_id = ? OR edges.to_id = ?) AND edge_edits.user_id != ?", ID, ID, user.Key).
			Count(&edges).Error; err != nil {
			return err
		}
		if edges >= 1 {
			return errors.New("cannot delete node with edges, remove edges first")
		}
		if err := tx.Delete(&Node{Model: gorm.Model{ID: atoi(ID)}}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Edge{}).
			Joins("JOIN edge_edits ON edges.id = edge_edits.edge_id").
			Where("(edges.from_id = ? OR edges.to_id = ?) AND edge_edits.user_id == ?", ID, ID, user.Key).
			Error; err != nil {
			return err
		}
		return tx.Where("node_id = ?", ID).Delete(&NodeEdit{}).Error
	}); err != nil {
		return errors.Wrap(err, "transaction failed")
	}
	return nil
}

func isUserAdmin(tx *gorm.DB, userID string) (bool, error) {
	var roleAdmin int64
	if err := tx.Model(&Role{}).Where("user_id = ? AND role = ?", userID, db.RoleAdmin).Count(&roleAdmin).Error; err != nil {
		return false, err
	}
	return roleAdmin == 1, nil
}

func (pg *PostgresDB) DeleteEdge(ctx context.Context, user db.User, ID string) error {
	if err := pg.db.Transaction(func(tx *gorm.DB) error {
		var (
			edits int64
		)
		if err := tx.Model(&EdgeEdit{}).Where("edge_id = ? AND user_id != ?", ID, user.Key).Count(&edits).Error; err != nil {
			return err
		}
		isAdmin, err := isUserAdmin(tx, user.Key)
		if err != nil {
			return err
		}
		if edits >= 1 && !isAdmin {
			return errors.New("edge has edits from other users, won't delete")
		}
		if err := tx.Unscoped().Delete(&Edge{Model: gorm.Model{ID: atoi(ID)}}).Error; err != nil {
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
			return errors.Wrap(err, "failed to fetch user")
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
	token := middleware.CtxGetAuthentication(ctx)
	userID := middleware.CtxGetUserID(ctx)
	if userID == "" {
		return errors.New("no userID in HTTP-header found")
	}
	user := User{Model: gorm.Model{ID: atoi(userID)}}
	if err := pg.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&user).Preload("Tokens").First(&user).Error; err != nil {
			return err
		}
		if db.FindFirst(user.Tokens, makeIsValidTokenFn(pg, token)) == nil {
			return errors.New("no token found")
		}
		return tx.Delete(&user).Error
	}); err != nil {
		return errors.Wrapf(err, "transaction failed: %#v", user)
	}
	return nil
}

func (pg *PostgresDB) NodeEdits(ctx context.Context, ID string) ([]*model.NodeEdit, error) {
	edits := []NodeEdit{}
	err := pg.db.Where("node_id = ?", ID).Preload("User").Find(&edits).Error
	if len(edits) == 0 {
		return nil, errors.Errorf("nodeedit for node.id='%s' does not exist", ID)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to query edits")
	}
	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).NodeEdits(edits), nil
}

func (pg *PostgresDB) EdgeEdits(ctx context.Context, ID string) ([]*model.EdgeEdit, error) {
	edits := []EdgeEdit{}
	if err := pg.db.Where("edge_id = ?", ID).Preload("User").Find(&edits).Error; err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.Errorf("edge with id='%s' does not exist", ID)
	}
	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).EdgeEdits(edits), nil
}
