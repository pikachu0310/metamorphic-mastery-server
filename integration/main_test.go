package integration

import (
	"github.com/pikachu0310/go-backend-template/internal/handler"
	"github.com/pikachu0310/go-backend-template/internal/migration"
	"github.com/pikachu0310/go-backend-template/internal/pkg/config"
	"github.com/pikachu0310/go-backend-template/internal/repository"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/ory/dockertest/v3"
)

var (
	db        *sqlx.DB
	e         *echo.Echo
	r         *repository.Repository
	h         *handler.Handler
	userIDMap = make(map[string]uuid.UUID)
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatal("connect to docker: ", err)
	}

	if err := pool.Client.Ping(); err != nil {
		log.Fatal("ping docker: ", err)
	}

	mysqlConfig := config.MySQL()

	resource, err := pool.Run("mysql", "latest", []string{
		"MYSQL_ROOT_PASSWORD=" + mysqlConfig.Passwd,
		"MYSQL_DATABASE=" + mysqlConfig.DBName,
	})
	if err != nil {
		log.Fatal("run docker: ", err)
	}

	mysqlConfig.Addr = "localhost:" + resource.GetPort("3306/tcp")

	if err := pool.Retry(func() error {
		_db, err := sqlx.Connect("mysql", mysqlConfig.FormatDSN())
		if err != nil {
			return err
		}
		db = _db

		return _db.Ping()
	}); err != nil {
		log.Fatal("connect to database container: ", err)
	}

	// migrate tables
	if err := migration.MigrateTables(db.DB); err != nil {
		log.Fatal("migrate tables: ", err)
	}

	// setup dependencies
	r = repository.New(db)
	h = handler.New(r)
	e = echo.New()
	h.SetupRoutes(e.Group("/api/v1"))

	log.Println("start integration test")
	m.Run()

	if err := pool.Purge(resource); err != nil {
		log.Fatal("purge docker: ", err)
	}
}

func doRequest(t *testing.T, method, path string, bodystr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(bodystr))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

func assert(t *testing.T, expected any, actual any) {
	t.Helper()

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("diff: %v", diff)
		t.Fail()
	}
}
