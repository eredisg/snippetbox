package main

import (
	"crypto/tls"
	"database/sql"
	"eredis.dev/snippetbox/pkg/models/mysql"
	"flag"
	"github.com/alexedwards/scs/v2"
	_ "github.com/go-sql-driver/mysql"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

type contextKey string

const contextKeyIsAuthenticated = contextKey("isAuthenticated")

type application struct {
	config 		  *Config
	errorLog      *log.Logger
	infoLog 	  *log.Logger
	session       *scs.SessionManager
	snippets 	  *mysql.SnippetModel
	templateCache map[string]*template.Template
	users		  *mysql.UserModel
}

type Config struct {
	Addr 	  string
	StaticDir string
	Dsn 	  string
}

func main() {
	config := new(Config)
	flag.StringVar(&config.Addr, "addr", ":4000", "HTTP network address")
	flag.StringVar(&config.StaticDir, "static-dir", "./ui/static/", "Path to static assets")
	flag.StringVar(&config.Dsn, "dsn", "web:pass@/snippetbox?parseTime=true", "MySQL data source name")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.Lshortfile)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	db, err := openDB(config.Dsn)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer db.Close()

	templateCache, err := newTemplateCache("./ui/html/")
	if err != nil {
		errorLog.Fatal(err)
	}

	session := scs.New()
	session.Lifetime = 12 * time.Hour
	session.Cookie.Secure = true
	session.Cookie.SameSite = http.SameSiteStrictMode

	app := &application{
		config: 	   config,
		errorLog: 	   errorLog,
		infoLog: 	   infoLog,
		session: 	   session,
		snippets: 	   &mysql.SnippetModel{DB: db},
		templateCache: templateCache,
		users:		   &mysql.UserModel{DB: db},
	}

	tlsConfig := &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences: 		  []tls.CurveID{tls.X25519, tls.CurveP256},
	}
	srv := &http.Server{
		Addr:	  config.Addr,
		ErrorLog: errorLog,
		Handler:  app.routes(),
		TLSConfig: tlsConfig,
		IdleTimeout: time.Minute,
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	infoLog.Printf("Starting server on %s", config.Addr)
	err = srv.ListenAndServeTLS("./tls/cert.pem", "./tls/key.pem")
	errorLog.Fatal(err)
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}