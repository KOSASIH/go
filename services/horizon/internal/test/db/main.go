// Package db provides helpers to connect to test databases.  It has no
// internal dependencies on horizon and so should be able to be imported by
// any horizon package.
package db

import (
	"fmt"
	"log"
	"testing"

	// pq enables postgres support
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	db "github.com/stellar/go/support/db/dbtest"
)

var (
	horizonDB     *db.DB
	coreDB        *db.DB
	coreDBConn    *sqlx.DB
	horizonDBConn *sqlx.DB
)

func horizonPostgres(t *testing.T) *db.DB {
	if horizonDB != nil {
		return horizonDB
	}
	horizonDB = db.Postgres(t)
	return horizonDB
}

// TODO, remove refs to internal core db, need to remove scenario tests which require this
// to seed core db.
func corePostgres(t *testing.T) *db.DB {
	if coreDB != nil {
		return coreDB
	}
	coreDB = db.Postgres(t)
	return coreDB
}

func Horizon(t *testing.T) *sqlx.DB {
	if horizonDBConn != nil {
		return horizonDBConn
	}

	horizonDBConn = horizonPostgres(t).Open()
	return horizonDBConn
}

func HorizonURL() string {
	if horizonDB == nil {
		log.Panic(fmt.Errorf("Horizon not initialized"))
	}
	return horizonDB.DSN
}

func HorizonROURL() string {
	if horizonDB == nil {
		log.Panic(fmt.Errorf("Horizon not initialized"))
	}
	return horizonDB.RO_DSN
}

// TODO, remove refs to core db, need to remove scenario tests which require this
// to seed core db.
func StellarCore(t *testing.T) *sqlx.DB {
	if coreDBConn != nil {
		return coreDBConn
	}
	coreDBConn = corePostgres(t).Open()
	return coreDBConn
}

// TODO, remove refs to core db, need to remove scenario tests which require this
// to seed core db.
func StellarCoreURL() string {
	if coreDB == nil {
		log.Panic(fmt.Errorf("StellarCore not initialized"))
	}
	return coreDB.DSN
}
