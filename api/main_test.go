package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func testAPI() *API { return &API{store: NewStore()} }
func call(t *testing.T, a *API, method, path, body string) (*httptest.ResponseRecorder, Record) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.ServeHTTP(rec, req)
	var payload Record
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, rec.Body.String())
	}
	return rec, payload
}
func TestHealthUsesStandardEnvelope(t *testing.T) {
	rec, p := call(t, testAPI(), http.MethodGet, "/api/v1/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	data, ok := p["data"].(map[string]any)
	if !ok || data["status"] != "ok" {
		t.Fatalf("unexpected data: %#v", p["data"])
	}
	meta, ok := p["meta"].(map[string]any)
	if !ok || meta["request_id"] == nil || meta["timestamp"] == nil {
		t.Fatalf("missing standard metadata: %#v", p["meta"])
	}
}
func TestListSearchAndPagination(t *testing.T) {
	rec, p := call(t, testAPI(), http.MethodGet, "/api/v1/orders?q=Demo+Customer+01&page=1&page_size=1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rows, ok := p["data"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("expected one matching order, got %#v", p["data"])
	}
	meta := p["meta"].(map[string]any)
	pagination := meta["pagination"].(map[string]any)
	if pagination["total"].(float64) != 1 {
		t.Fatalf("unexpected pagination: %#v", pagination)
	}
}
func TestCreateCustomerAndValidation(t *testing.T) {
	a := testAPI()
	rec, p := call(t, a, http.MethodPost, "/api/v1/customers", `{"name":"Demo Created","phone":"+255 700 000 010","email":"demo.created@example.test"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	d := p["data"].(map[string]any)
	id := d["id"].(string)
	if !strings.HasPrefix(id, "CUS-") {
		t.Fatalf("unexpected id: %s", id)
	}
	rec, p = call(t, a, http.MethodPost, "/api/v1/customers", `{"name":"No phone"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	if p["error"].(map[string]any)["code"] != "VALIDATION_ERROR" {
		t.Fatalf("unexpected error: %#v", p["error"])
	}
}
func TestPatchAndDeleteOrder(t *testing.T) {
	a := testAPI()
	rec, p := call(t, a, http.MethodPatch, "/api/v1/orders/ORD-0248", `{"status":"Completed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p["data"].(map[string]any)["status"] != "Completed" {
		t.Fatalf("patch not applied: %#v", p["data"])
	}
	rec, p = call(t, a, http.MethodDelete, "/api/v1/orders/ORD-0248", "")
	if rec.Code != http.StatusOK || p["data"].(map[string]any)["deleted"] != true {
		t.Fatalf("delete failed: %d %#v", rec.Code, p)
	}
	rec, p = call(t, a, http.MethodGet, "/api/v1/orders/ORD-0248", "")
	if rec.Code != http.StatusNotFound || p["error"].(map[string]any)["code"] != "NOT_FOUND" {
		t.Fatalf("expected not found: %d %#v", rec.Code, p)
	}
}
func TestPaymentLedgerIsDerivedAndReadOnly(t *testing.T) {
	a := testAPI()
	rec, p := call(t, a, http.MethodGet, "/api/v1/payments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	rows := p["data"].([]any)
	if len(rows) == 0 {
		t.Fatal("expected seeded payment ledger")
	}
	row := rows[0].(map[string]any)
	if row["currency"] != "TZS" || row["due"] == nil {
		t.Fatalf("unexpected ledger row: %#v", row)
	}
	rec, p = call(t, a, http.MethodPost, "/api/v1/payments", `{}`)
	if rec.Code != http.StatusMethodNotAllowed || p["error"].(map[string]any)["code"] != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected 405: %d %#v", rec.Code, p)
	}
}

func TestDesktopHTTPHandlerPersistsRecordsWithoutOnlineIdentity(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "simamia.db")
	handler, closeStore, err := NewDesktopHTTPHandler(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/customers", strings.NewReader(`{"name":"Offline customer","phone":"+255 700 000 099"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("offline create status=%d body=%s", response.Code, response.Body.String())
	}
	if err := closeStore(); err != nil {
		t.Fatal(err)
	}

	handler, closeStore, err = NewDesktopHTTPHandler(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/customers?q=Offline+customer", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Offline customer") {
		t.Fatalf("offline record was not restored: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPersistentServerHandlerRequiresLocalAccountSession(t *testing.T) {
	handler, closeStore, err := NewHTTPHandler(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected server handler to require a local account session, got %d: %s", response.Code, response.Body.String())
	}
}

func callHandler(t *testing.T, handler http.Handler, method, path, body, token string) (*httptest.ResponseRecorder, Record) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var payload Record
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, response.Body.String())
	}
	return response, payload
}

func authToken(t *testing.T, payload Record) string {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing response data: %#v", payload)
	}
	token, ok := data["access_token"].(string)
	if !ok || len(token) < 32 {
		t.Fatalf("missing valid access token: %#v", data)
	}
	return token
}

func TestLocalAccountRegistrationLoginAndSessionPersistence(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "accounts.db")
	handler, closeStore, err := NewHTTPHandler(databasePath)
	if err != nil {
		t.Fatal(err)
	}

	response, payload := callHandler(t, handler, http.MethodPost, "/api/v1/auth/register", `{"name":"Asha Mushi","phone":"0712 345 678","password":"local-pass-123","remember_me":true}`, "")
	if response.Code != http.StatusOK {
		t.Fatalf("registration failed: %d %s", response.Code, response.Body.String())
	}
	token := authToken(t, payload)
	data := payload["data"].(map[string]any)
	user := data["user"].(map[string]any)
	if user["name"] != "Asha Mushi" || user["phone_number"] != "+255712345678" {
		t.Fatalf("unexpected local user: %#v", user)
	}

	response, _ = callHandler(t, handler, http.MethodGet, "/api/v1/auth/me", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("session validation failed: %d %s", response.Code, response.Body.String())
	}
	response, duplicate := callHandler(t, handler, http.MethodPost, "/api/v1/auth/register", `{"name":"Asha Mushi","phone":"+255 712 345 678","password":"local-pass-123"}`, "")
	if response.Code != http.StatusConflict || duplicate["error"].(map[string]any)["code"] != "ACCOUNT_EXISTS" {
		t.Fatalf("duplicate phone should conflict: %d %#v", response.Code, duplicate)
	}

	response, login := callHandler(t, handler, http.MethodPost, "/api/v1/auth/login", `{"phone":"+255712345678","password":"wrong-password"}`, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password should be rejected, got %d", response.Code)
	}
	_ = login
	response, login = callHandler(t, handler, http.MethodPost, "/api/v1/auth/login", `{"phone":"0712345678","password":"local-pass-123","remember_me":true}`, "")
	if response.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", response.Code, response.Body.String())
	}
	token = authToken(t, login)

	if err := closeStore(); err != nil {
		t.Fatal(err)
	}
	handler, closeStore, err = NewHTTPHandler(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	response, _ = callHandler(t, handler, http.MethodGet, "/api/v1/auth/me", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("session did not survive API restart: %d %s", response.Code, response.Body.String())
	}
	response, _ = callHandler(t, handler, http.MethodPost, "/api/v1/auth/logout", `{}`, token)
	if response.Code != http.StatusOK {
		t.Fatalf("logout failed: %d %s", response.Code, response.Body.String())
	}
	response, _ = callHandler(t, handler, http.MethodGet, "/api/v1/auth/me", "", token)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out token remained valid: %d", response.Code)
	}
}

func TestLocalAccountsCannotReadOtherAccountsRecords(t *testing.T) {
	handler, closeStore, err := NewHTTPHandler(filepath.Join(t.TempDir(), "isolation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	_, first := callHandler(t, handler, http.MethodPost, "/api/v1/auth/register", `{"name":"First Owner","phone":"712111111","password":"local-pass-123"}`, "")
	_, second := callHandler(t, handler, http.MethodPost, "/api/v1/auth/register", `{"name":"Second Owner","phone":"712222222","password":"local-pass-456"}`, "")
	firstToken, secondToken := authToken(t, first), authToken(t, second)

	response, created := callHandler(t, handler, http.MethodPost, "/api/v1/customers", `{"name":"Private customer","phone":"+255 700 000 010"}`, firstToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("could not create owner record: %d %s", response.Code, response.Body.String())
	}
	response, firstList := callHandler(t, handler, http.MethodGet, "/api/v1/customers", "", firstToken)
	if response.Code != http.StatusOK || len(firstList["data"].([]any)) != 1 {
		t.Fatalf("owner cannot read its own record: %d %#v", response.Code, firstList)
	}
	response, secondList := callHandler(t, handler, http.MethodGet, "/api/v1/customers", "", secondToken)
	if response.Code != http.StatusOK || len(secondList["data"].([]any)) != 0 {
		t.Fatalf("record was visible to another account: %d %#v %#v", response.Code, secondList, created)
	}
}

func TestLocalAccountRegistrationRejectsWeakPasswordsAndBadPhones(t *testing.T) {
	handler, closeStore, err := NewHTTPHandler(filepath.Join(t.TempDir(), "validation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	for _, input := range []string{
		`{"name":"Asha Mushi","phone":"712345678","password":"short"}`,
		`{"name":"Asha Mushi","phone":"123","password":"local-pass-123"}`,
	} {
		response, payload := callHandler(t, handler, http.MethodPost, "/api/v1/auth/register", input, "")
		if response.Code != http.StatusUnprocessableEntity || payload["error"].(map[string]any)["code"] != "VALIDATION_ERROR" {
			t.Fatalf("invalid registration was accepted: %d %#v", response.Code, payload)
		}
	}
}
