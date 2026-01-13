package models

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/ini.v1"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var err error

func init() {
	path := os.Getenv("APP_CONFIG_PATH")
	if path == "" {
		path = "./config/orderbook.ini"
	}
	cfg, err := ini.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	/**
	  host     = 127.0.0.1
	  port     = 3306
	  user     = root
	  password = root
	  database = gin
	  **/

	host := cfg.Section("mysql").Key("host").String()
	port := cfg.Section("mysql").Key("port").String()
	user := cfg.Section("mysql").Key("user").String()
	pw := cfg.Section("mysql").Key("password").String()
	db := cfg.Section("mysql").Key("database").String()

	// dsn := "root:root@tcp(127.0.0.1:3306)/gin?charset=utf8mb4&parseTime=True&loc=Local"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pw, host, port, db)
	/**
	在 GORM 中，默认的日志隔离级别是 logger.Silent，这意味着如果没有特别配置，GORM 是不会打印任何日志的，
	包括 SQL 查询。也就是说，如果你没有明确设置日志级别为 Info 或 Debug，默认情况下 SQL 查询和其他日志信息不会被打印。
	logger.Silent: 不打印任何日志。
	logger.Error: 只打印错误日志。
	logger.Warn: 打印警告日志。
	logger.Info: 打印信息日志，包括 SQL 查询。
	logger.Debug: 打印调试日志，包含 SQL 查询和更多的细节。
	*/
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,                                // 禁用默认事务
		QueryFields:            true,                                // 打印sql
		Logger:                 logger.Default.LogMode(logger.Info), // 设置日志级别为Info，确保打印SQL
	})
	if err != nil {
		log.Fatal(err)
	}
}
