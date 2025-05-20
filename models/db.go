package models

import (
	"billing-job/config"
	"billing-job/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB
var dbhost string
var dbuser string
var dbpassword string

// set connection to db.
func init() {
	dbhost = config.DB.Host
	dbuser = config.DB.User
	dbpassword = config.DB.Password

	dbport := config.DB.Port
	dbname := config.DB.Name

	// Maximum number of idle and active connections
	dbmaxIdle := 50
	dbmaxConn := 50

	// Connection string for MySQL (format: user:password@tcp(host:port)/dbname?charset=utf8)
	dsn := dbuser + ":" + dbpassword + "@tcp(" + dbhost + ":" + dbport + ")/" + dbname + "?charset=utf8&parseTime=true"

	// Initialize GORM MySQL connection
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.SugarLogger.Fatal("Failed to connect to MySQL: %v", err)
	}

	// Configure connection pool settings
	sqlDB, err := DB.DB()
	if err != nil {
		log.SugarLogger.Fatal("Failed to get DB instance from GORM: %v", err)
	}

	// Set connection pool limits
	sqlDB.SetMaxIdleConns(dbmaxIdle)
	sqlDB.SetMaxOpenConns(dbmaxConn)
}
