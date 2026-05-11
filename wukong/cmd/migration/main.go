package main

import (
	"context"
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jiujuan/wukong/pkg/config"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("migration", flag.ExitOnError)
	configPath := fs.String("config", "configs/dev.yaml", "config file path")
	dsn := fs.String("db", "", "database URL, overrides config and DATABASE_URL")
	dir := fs.String("dir", "migrations", "embedded migrations directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	commandArgs := fs.Args()
	if len(commandArgs) == 0 {
		commandArgs = []string{"status"}
	}

	connString := strings.TrimSpace(*dsn)
	if connString == "" {
		connString = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if connString == "" {
		cfg, err := config.New(config.WithConfigPath(*configPath))
		if err != nil {
			return err
		}
		connString = dsnFromConfig(cfg)
	}

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	goose.SetBaseFS(migrations)
	goose.SetTableName("goose_db_version")
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.RunContext(ctx, commandArgs[0], db, *dir, commandArgs[1:]...)
}

func dsnFromConfig(cfg *config.Config) string {
	user := cfg.String("db.user", "wukong")
	password := cfg.String("db.password", "wukong123")
	host := cfg.String("db.host", "localhost")
	port := cfg.Int("db.port", 5432)
	database := cfg.String("db.database", "wukong")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + strconv.Itoa(port),
		Path:   database,
	}
	query := u.Query()
	query.Set("sslmode", "disable")
	u.RawQuery = query.Encode()
	return u.String()
}
