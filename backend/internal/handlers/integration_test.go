package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/anshika/taskflow/internal/db"
	"github.com/anshika/taskflow/internal/router"
)

const testJWTSecret = "integration-test-jwt-secret-do-not-use-in-prod"

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL (e.g. postgres://postgres:postgres@localhost:5432/taskflow?sslmode=disable) to run integration tests")
	}
	return dsn
}

func resetTables(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := testDSN(t)
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `TRUNCATE TABLE tasks, projects, users CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
}

func testServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := testDSN(t)

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(dsn); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE TABLE tasks, projects, users CASCADE`)
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}

	api := &API{Pool: pool, JWTSecret: testJWTSecret}
	ts := httptest.NewServer(router.New(api))
	return ts, func() {
		ts.Close()
		pool.Close()
	}
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	testDSN(t)
	resetTables(t, ctx)

	ts, cleanup := testServer(t)
	defer cleanup()

	body := map[string]string{"name": "A", "email": "a@b.com", "password": "password123"}
	b, _ := json.Marshal(body)
	res, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", res.StatusCode)
	}

	res2, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate register: %d", res2.StatusCode)
	}
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	testDSN(t)
	resetTables(t, ctx)

	ts, cleanup := testServer(t)
	defer cleanup()

	reg := map[string]string{"name": "A", "email": "a@b.com", "password": "password123"}
	rb, _ := json.Marshal(reg)
	if _, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(rb)); err != nil {
		t.Fatal(err)
	}

	okBody := map[string]string{"email": "a@b.com", "password": "password123"}
	b, _ := json.Marshal(okBody)
	res, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login ok: %d", res.StatusCode)
	}

	bad := map[string]string{"email": "a@b.com", "password": "wrong"}
	b2, _ := json.Marshal(bad)
	res2, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewReader(b2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login bad password: %d", res2.StatusCode)
	}
}

func TestCreateTaskAuth(t *testing.T) {
	ctx := context.Background()
	testDSN(t)
	resetTables(t, ctx)

	ts, cleanup := testServer(t)
	defer cleanup()
	client := &http.Client{}

	reg := map[string]string{"name": "Owner", "email": "o@b.com", "password": "password123"}
	rb, _ := json.Marshal(reg)
	res, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewReader(rb))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var regOut struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&regOut); err != nil {
		t.Fatal(err)
	}

	pid := "00000000-0000-0000-0000-000000000099"
	reqNoAuth, _ := http.NewRequest(http.MethodPost, ts.URL+"/projects/"+pid+"/tasks", bytes.NewReader([]byte(`{"title":"x"}`)))
	reqNoAuth.Header.Set("Content-Type", "application/json")
	resNoAuth, err := client.Do(reqNoAuth)
	if err != nil {
		t.Fatal(err)
	}
	defer resNoAuth.Body.Close()
	if resNoAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no auth: %d", resNoAuth.StatusCode)
	}

	pbody := map[string]string{"name": "P"}
	pb, _ := json.Marshal(pbody)
	reqP, _ := http.NewRequest(http.MethodPost, ts.URL+"/projects", bytes.NewReader(pb))
	reqP.Header.Set("Content-Type", "application/json")
	reqP.Header.Set("Authorization", "Bearer "+regOut.Token)
	pr, err := client.Do(reqP)
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Body.Close()
	if pr.StatusCode != http.StatusCreated {
		t.Fatalf("create project: %d", pr.StatusCode)
	}
	var proj struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(pr.Body).Decode(&proj); err != nil {
		t.Fatal(err)
	}

	tbody := map[string]string{"title": "Task 1"}
	tb, _ := json.Marshal(tbody)
	reqT, _ := http.NewRequest(http.MethodPost, ts.URL+"/projects/"+proj.ID+"/tasks", bytes.NewReader(tb))
	reqT.Header.Set("Content-Type", "application/json")
	reqT.Header.Set("Authorization", "Bearer "+regOut.Token)
	tr, err := client.Do(reqT)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Body.Close()
	if tr.StatusCode != http.StatusCreated {
		t.Fatalf("create task: %d", tr.StatusCode)
	}
}
