package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	"net"
	"net/url"
	"os"
	"sqlterm/internal/config"
	"strconv"
	"strings"
	"sync/atomic"
)

var tlsID atomic.Uint64

func Connect(ctx context.Context, p config.Profile, password string) (*Session, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	port := p.EffectivePort()
	addr := net.JoinHostPort(p.Host, strconv.Itoa(port))
	var driver, dsn string
	mode := p.TLSMode()
	switch p.Engine {
	case "sqlserver":
		driver = "sqlserver"
		dsn = sqlServerDSN(p, password, addr, mode)
	case "postgres":
		driver = "pgx"
		u := url.URL{Scheme: "postgres", Host: addr, User: url.UserPassword(p.User, password), Path: "/" + p.Database}
		q := url.Values{}
		ssl := "verify-full"
		if mode == "disable" {
			ssl = "disable"
		}
		if mode == "insecure" {
			ssl = "require"
		}
		q.Set("sslmode", ssl)
		q.Set("application_name", "SQLTerm")
		if p.CAFile != "" && mode == "verify" {
			q.Set("sslrootcert", p.CAFile)
		}
		u.RawQuery = q.Encode()
		dsn = u.String()
	case "mysql":
		driver = "mysql"
		c := mysql.NewConfig()
		c.User = p.User
		c.Passwd = password
		c.Net = "tcp"
		c.Addr = addr
		c.DBName = p.Database
		c.ParseTime = true
		if mode != "disable" {
			tc := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: p.Host, InsecureSkipVerify: mode == "insecure"}
			if p.CAFile != "" && mode == "verify" {
				b, e := os.ReadFile(p.CAFile)
				if e != nil {
					return nil, fmt.Errorf("read CA: %w", e)
				}
				pool, e := x509.SystemCertPool()
				if e != nil {
					pool = x509.NewCertPool()
				}
				if !pool.AppendCertsFromPEM(b) {
					return nil, fmt.Errorf("invalid CA certificate")
				}
				tc.RootCAs = pool
			}
			name := fmt.Sprintf("sqlterm-%d", tlsID.Add(1))
			if e := mysql.RegisterTLSConfig(name, tc); e != nil {
				return nil, e
			}
			defer mysql.DeregisterTLSConfig(name)
			c.TLSConfig = name
		}
		dsn = c.FormatDSN()
	}
	var database *sql.DB
	var e error
	messages := make(chan string, 256)
	if p.Engine == "postgres" {
		var pc *pgx.ConnConfig
		pc, e = pgx.ParseConfig(dsn)
		if e == nil {
			pc.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
				select {
				case messages <- n.Message:
				default:
				}
			}
			database = stdlib.OpenDB(*pc)
		}
	} else {
		database, e = sql.Open(driver, dsn)
	}
	if e != nil {
		return nil, redact(e, password, dsn)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	ctx, cancel := deadline(ctx)
	defer cancel()
	conn, e := database.Conn(ctx)
	if e == nil {
		e = conn.PingContext(ctx)
	}
	if e != nil {
		if conn != nil {
			conn.Close()
		}
		database.Close()
		return nil, redact(e, password, dsn)
	}
	currentSQL := "SELECT current_database()"
	if p.Engine == "mysql" {
		currentSQL = "SELECT DATABASE()"
	}
	if p.Engine == "sqlserver" {
		currentSQL = "SELECT DB_NAME()"
	}
	var currentDB sql.NullString
	if e = conn.QueryRowContext(ctx, currentSQL).Scan(&currentDB); e != nil {
		conn.Close()
		database.Close()
		return nil, redact(e, password, dsn)
	}
	if currentDB.Valid {
		p.Database = currentDB.String
	}
	return &Session{DB: database, Conn: conn, Adapter: NewAdapter(p.Engine), Profile: p, Messages: messages}, nil
}
func sqlServerDSN(p config.Profile, password, addr, mode string) string {
	u := url.URL{Scheme: "sqlserver", Host: addr, User: url.UserPassword(p.User, password)}
	q := url.Values{}
	q.Set("database", p.Database)
	q.Set("app name", "SQLTerm")
	if mode == "disable" {
		q.Set("encrypt", "disable")
	} else {
		q.Set("encrypt", "true")
		q.Set("TrustServerCertificate", strconv.FormatBool(mode == "insecure"))
	}
	if p.CAFile != "" && mode == "verify" {
		q.Set("certificate", p.CAFile)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
func redact(e error, secrets ...string) error {
	s := e.Error()
	for _, v := range secrets {
		if v != "" {
			s = strings.ReplaceAll(s, v, "[redacted]")
		}
	}
	return fmt.Errorf("%s", s)
}
