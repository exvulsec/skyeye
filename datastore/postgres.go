package datastore

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"go-etl/config"
)

var dbInstance *DBInstance
var pgxInstance *PGXInstance

type DBInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

type PGXInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

// Instance gets the singleton instance
func (i *DBInstance) Instance() any {
	i.once.Do(func() {
		i.instance = i.initializer()
	})
	return i.instance
}

func (i *PGXInstance) Instance() any {
	i.once.Do(func() {
		i.instance = i.initializer()
	})
	return i.instance
}

func initPostgresql() any {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		config.Conf.Postgresql.Host,
		config.Conf.Postgresql.Port,
		config.Conf.Postgresql.User,
		config.Conf.Postgresql.Password,
		config.Conf.Postgresql.Database,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.Panicf("connect to postgresql is err: %v", err)
		return nil
	}

	stdDB, _ := db.DB()
	stdDB.SetMaxOpenConns(config.Conf.Postgresql.MaxOpenConns)
	stdDB.SetMaxIdleConns(config.Conf.Postgresql.MaxIdleConns)

	if config.Conf.Postgresql.LogMode {
		db.Debug()
	}

	logrus.Infof("connect to postgresql successfully")
	return db
}

func initPGX() any {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		config.Conf.Postgresql.Host,
		config.Conf.Postgresql.Port,
		config.Conf.Postgresql.User,
		config.Conf.Postgresql.Password,
		config.Conf.Postgresql.Database,
	)

	conn, err := pgxpool.New(context.Background(), dsn)
	conn.Config().MaxConns = int32(config.Conf.Postgresql.MaxOpenConns)
	if err != nil {
		logrus.Panicf("connect to postgresql is err: %v", err)
		return nil
	}

	logrus.Infof("connect to postgresql by pgx is successfully")
	return conn
}

func DB() *gorm.DB {
	return dbInstance.Instance().(*gorm.DB)
}

func PGX() *pgxpool.Pool {
	return pgxInstance.Instance().(*pgxpool.Pool)
}

func init() {
	dbInstance = &DBInstance{initializer: initPostgresql}
	pgxInstance = &PGXInstance{initializer: initPGX}
}
