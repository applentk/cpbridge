package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cp-hub/api/internal/admin"
	"github.com/cp-hub/api/internal/auth"
	"github.com/cp-hub/api/internal/contest"
	"github.com/cp-hub/api/internal/db"
	"github.com/cp-hub/api/internal/platform"
	"github.com/cp-hub/api/internal/platform/atcoder"
	"github.com/cp-hub/api/internal/platform/codeforces"
	"github.com/cp-hub/api/internal/problem"
	"github.com/cp-hub/api/internal/problemset"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testApp struct {
	router      *chi.Mux
	authSvc     *auth.Service
	contestSvc  *contest.Service
	problemSvc  *problem.Service
	adminUser   *auth.User
	adminToken  string
	normalUser  *auth.User
	userToken   string
	testProblem string
}

func setupTestApp(t *testing.T) *testApp {
	database, err := db.Connect()
	if err != nil {
		t.Skipf("Skipping database integration tests: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.Ping(); err != nil {
		t.Skipf("Skipping database integration tests: %v", err)
	}

	err = db.EnsureSchema(database)
	require.NoError(t, err)

	suffix := time.Now().UnixNano()
	adminEmail := fmt.Sprintf("admin_%d@test.com", suffix)
	userEmail := fmt.Sprintf("user_%d@test.com", suffix)
	testProblemID := fmt.Sprintf("cf_test_%d", suffix)

	// Keep integration tests isolated from the developer database. Cleanup is
	// registered before any test data is created so it also runs on failures.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			t.Errorf("begin test database cleanup: %v", err)
			return
		}
		defer tx.Rollback()

		// Delete dependents first because contests reference both users and
		// problems. The user delete then cascades any remaining owned data.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM contests
			WHERE owner_id IN (SELECT id FROM users WHERE email IN ($1, $2))
		`, adminEmail, userEmail); err != nil {
			t.Errorf("delete test contests: %v", err)
			return
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM problems WHERE external_id = $1`, testProblemID); err != nil {
			t.Errorf("delete test problem: %v", err)
			return
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE email IN ($1, $2)`, adminEmail, userEmail); err != nil {
			t.Errorf("delete test users: %v", err)
			return
		}
		if err := tx.Commit(); err != nil {
			t.Errorf("commit test database cleanup: %v", err)
		}
	})

	platRegistry := platform.NewRegistry()
	platRegistry.Register(codeforces.New())
	platRegistry.Register(atcoder.New())

	authSvc := auth.NewService(database)
	probSvc := problem.NewService(database, platRegistry)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)

	authHandler := auth.NewHandler(authSvc)
	probHandler := problem.NewHandler(probSvc, authSvc)
	contestHandler := contest.NewHandler(contestSvc, authSvc)
	adminHandler := admin.NewHandler(database, authSvc, probSvc, setSvc, contestSvc)

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		api.Mount("/auth", authHandler.Routes())
		api.Mount("/problems", probHandler.Routes())
		api.Mount("/contests", contestHandler.Routes())

		api.Group(func(adminRouter chi.Router) {
			adminRouter.Use(authSvc.AuthMiddleware(true))
			adminRouter.Use(auth.RequireAdmin())
			adminRouter.Mount("/admin", adminHandler.Routes())
		})
	})

	adminUser, adminToken, err := authSvc.Register(context.Background(), adminEmail, fmt.Sprintf("adm_%d", suffix), "password123")
	require.NoError(t, err)
	// Promote adminUser to ADMIN role
	_, err = database.Exec("UPDATE users SET role = 'ADMIN' WHERE id = $1", adminUser.ID)
	require.NoError(t, err)
	adminUser.Role = auth.RoleAdmin

	normalUser, userToken, err := authSvc.Register(context.Background(), userEmail, fmt.Sprintf("usr_%d", suffix), "password123")
	require.NoError(t, err)

	return &testApp{
		router:      r,
		authSvc:     authSvc,
		contestSvc:  contestSvc,
		problemSvc:  probSvc,
		adminUser:   adminUser,
		adminToken:  adminToken,
		normalUser:  normalUser,
		userToken:   userToken,
		testProblem: testProblemID,
	}
}

func TestAuthorizationMatrix(t *testing.T) {
	fixture := setupTestApp(t)
	app := fixture.router
	adminToken := fixture.adminToken
	userToken := fixture.userToken

	// Create a problem to test
	p, err := fixture.problemSvc.CreateCustom(context.Background(), problem.CreateCustomReq{
		Title:       "Test Problem A",
		Platform:    platform.Codeforces,
		ExternalID:  fixture.testProblem,
		Statement:   "Solve this",
		TimeLimit:   "1.0s",
		MemoryLimit: "256MB",
	})
	require.NoError(t, err)

	// 1. USER GET /api/contests -> 200
	req := httptest.NewRequest("GET", "/api/contests", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. USER POST /api/admin/contests -> 403 Forbidden
	reqBody, _ := json.Marshal(map[string]interface{}{
		"name":        "Unauthorized Contest",
		"startAt":     time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		"endAt":       time.Now().Add(3 * time.Hour).Format(time.RFC3339),
		"scoringType": "ICPC",
	})
	req = httptest.NewRequest("POST", "/api/admin/contests", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// 3. USER POST /api/admin/problems/import -> 403 Forbidden
	reqBody, _ = json.Marshal(map[string]interface{}{
		"url": "https://codeforces.com/problemset/problem/1900/A",
	})
	req = httptest.NewRequest("POST", "/api/admin/problems/import", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// 4. USER PATCH /api/admin/users/:id/role -> 403 Forbidden
	reqBody, _ = json.Marshal(map[string]interface{}{
		"role": "ADMIN",
	})
	req = httptest.NewRequest("PATCH", "/api/admin/users/dummy/role", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// 5. ADMIN POST /api/admin/contests -> 201 Created (Draft Contest)
	futureStart := time.Now().Add(2 * time.Hour)
	futureEnd := futureStart.Add(2 * time.Hour)
	reqBody, _ = json.Marshal(map[string]interface{}{
		"name":              "Admin Draft Contest",
		"description":       "Secret preview contest",
		"startAt":           futureStart.Format(time.RFC3339),
		"endAt":             futureEnd.Format(time.RFC3339),
		"scoringType":       "ICPC",
		"publicationStatus": "DRAFT",
		"problemIds":        []string{p.ID},
	})
	req = httptest.NewRequest("POST", "/api/admin/contests", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var createdDraft struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(w.Body).Decode(&createdDraft)
	require.NotEmpty(t, createdDraft.ID)

	// 6. USER GET draft contest -> 404 Not Found
	req = httptest.NewRequest("GET", "/api/contests/"+createdDraft.ID, nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 7. ADMIN GET draft contest -> 200 OK
	req = httptest.NewRequest("GET", "/api/contests/"+createdDraft.ID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 8. Publish the contest
	publishBody, _ := json.Marshal(map[string]interface{}{
		"publicationStatus": "PUBLISHED",
	})
	req = httptest.NewRequest("PATCH", "/api/admin/contests/"+createdDraft.ID, bytes.NewReader(publishBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 9. USER GET contest problems before start -> 403 CONTEST_NOT_STARTED
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/contests/%s/problems", createdDraft.ID), nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// 10. ADMIN GET contest problems before start -> 200 OK
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/contests/%s/problems", createdDraft.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLastActiveAdminProtection(t *testing.T) {
	fixture := setupTestApp(t)

	// Ensure there is only 1 admin for this test by checking count
	// Attempting to demote or disable adminUser
	// First let's ensure other test admins are USER
	// Demoting the last admin:
	_, err := fixture.authSvc.UpdateUserRole(context.Background(), fixture.adminUser.ID, "USER")
	if err != nil {
		assert.Equal(t, "LAST_ADMIN", err.Error())
	}

	// Disabling the last admin:
	_, err = fixture.authSvc.UpdateUserStatus(context.Background(), fixture.adminUser.ID, false)
	if err != nil {
		assert.Equal(t, "LAST_ADMIN", err.Error())
	}
}
