package handler

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"gamehub/store"
)

type sessionResp struct {
	Token  string       `json:"token"`
	Member store.Member `json:"member"`
}

func TestRegisterSuccess(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "  New.User@Example.COM ", "password": "supersecret", "displayName": "  Neo  ",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body)
	}
	got := decode[sessionResp](t, rec)
	if got.Token == "" {
		t.Error("register did not return a token")
	}
	if got.Member.Email != "new.user@example.com" || got.Member.DisplayName != "Neo" {
		t.Errorf("member = %+v, want normalized email and trimmed name", got.Member)
	}
	if got.Member.Role != "member" {
		t.Errorf("role = %q, want member", got.Member.Role)
	}
	// ห้ามมีรหัสผ่านหรือลายนิ้วมือรหัสผ่านรั่วออกไปกับ JSON
	for _, leak := range []string{"supersecret", "passwordHash", "password_hash", "$2a$"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("response leaks %q: %s", leak, rec.Body)
		}
	}
	// ตั๋วที่ได้ต้องใช้งานได้จริง
	if me := env.do(t, "GET", "/api/me", got.Token, nil); me.Code != http.StatusOK {
		t.Errorf("token from register rejected by /api/me: %d", me.Code)
	}
}

func TestRegisterIgnoresRoleFromClient(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "sneaky@example.com", "password": "supersecret", "displayName": "Sneaky", "role": "admin",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := decode[sessionResp](t, rec).Member.Role; got != "member" {
		t.Fatalf("role = %q, a client must not be able to self-assign admin", got)
	}
}

func TestRegisterValidation(t *testing.T) {
	ok := map[string]string{"email": "a@example.com", "password": "supersecret", "displayName": "A"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for key, val := range ok {
			m[key] = val
		}
		m[k] = v
		return m
	}
	tests := []struct {
		name string
		body any
		want string
	}{
		{"empty email", with("email", ""), "BAD_EMAIL"},
		{"no at sign", with("email", "not-an-email"), "BAD_EMAIL"},
		{"display-name form email", with("email", "Bob <b@example.com>"), "BAD_EMAIL"},
		{"too long email", with("email", strings.Repeat("a", 250)+"@x.com"), "BAD_EMAIL"},
		{"empty name", with("displayName", "   "), "BAD_NAME"},
		{"name too long", with("displayName", strings.Repeat("ก", 51)), "BAD_NAME"},
		{"short password", with("password", "1234567"), "BAD_PASSWORD"},
		{"password too long", with("password", strings.Repeat("a", 73)), "BAD_PASSWORD"},
		{"broken json", "{not json", "BAD_JSON"},
	}
	for _, tt := range tests {
		env := newTestEnv(t)
		rec := env.do(t, "POST", "/api/auth/register", "", tt.body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", tt.name, rec.Code)
			continue
		}
		if got := errCode(t, rec); got != tt.want {
			t.Errorf("%s: code = %q, want %q", tt.name, got, tt.want)
		}
		if len(env.members.byID) != 0 {
			t.Errorf("%s: a member was created despite invalid input", tt.name)
		}
	}
}

func TestRegisterRejectsOversizedBody(t *testing.T) {
	rec := newTestEnv(t).do(t, "POST", "/api/auth/register", "", `{"email":"`+strings.Repeat("a", 2<<20)+`"}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	env := newTestEnv(t)
	env.members.add(t, "taken@example.com", "supersecret", "member")
	rec := env.do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "TAKEN@example.com", "password": "supersecret", "displayName": "Dup",
	})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "EMAIL_TAKEN" {
		t.Fatalf("status = %d, want 409 EMAIL_TAKEN; body %s", rec.Code, rec.Body)
	}
}

func TestRegisterDatabaseFailureIsHidden(t *testing.T) {
	env := newTestEnv(t)
	env.members.failAll = errors.New("connection refused: secret-host:5432")
	rec := env.do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "a@example.com", "password": "supersecret", "displayName": "A",
	})
	if rec.Code != http.StatusInternalServerError || errCode(t, rec) != "INTERNAL" {
		t.Fatalf("status = %d, want 500 INTERNAL", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret-host") {
		t.Error("internal error details leaked to the client")
	}
}

func TestLoginSuccess(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "me@example.com", "supersecret", "member")
	rec := env.do(t, "POST", "/api/auth/login", "", map[string]string{
		"email": " ME@Example.com ", "password": "supersecret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	got := decode[sessionResp](t, rec)
	if got.Member.ID != m.ID {
		t.Errorf("member id = %d, want %d", got.Member.ID, m.ID)
	}
	id, version, err := env.tickets.Parse(got.Token)
	if err != nil || id != m.ID || version != m.TokenVersion {
		t.Errorf("token parsed as (%d, %d, %v), want (%d, %d, nil)", id, version, err, m.ID, m.TokenVersion)
	}
	if strings.Contains(rec.Body.String(), "$2a$") {
		t.Error("password hash leaked in login response")
	}
}

// ต้องตอบเหมือนกันทุกประการระหว่าง "ไม่มีอีเมลนี้" กับ "รหัสผิด" ไม่งั้นคนภายนอกเดาอีเมลในระบบได้
func TestLoginDoesNotRevealWhichEmailsExist(t *testing.T) {
	env := newTestEnv(t)
	env.members.add(t, "real@example.com", "supersecret", "member")

	wrongPass := env.do(t, "POST", "/api/auth/login", "", map[string]string{"email": "real@example.com", "password": "wrong-password"})
	noUser := env.do(t, "POST", "/api/auth/login", "", map[string]string{"email": "ghost@example.com", "password": "wrong-password"})

	if wrongPass.Code != http.StatusUnauthorized || noUser.Code != http.StatusUnauthorized {
		t.Fatalf("statuses = %d and %d, want both 401", wrongPass.Code, noUser.Code)
	}
	if wrongPass.Body.String() != noUser.Body.String() {
		t.Errorf("responses differ:\n wrong password: %s\n unknown email:  %s", wrongPass.Body, noUser.Body)
	}
	if errCode(t, wrongPass) != "BAD_CREDENTIALS" {
		t.Errorf("code = %q, want BAD_CREDENTIALS", errCode(t, wrongPass))
	}
}

func TestLoginBadJSON(t *testing.T) {
	rec := newTestEnv(t).do(t, "POST", "/api/auth/login", "", "nope")
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "BAD_JSON" {
		t.Fatalf("status = %d, want 400 BAD_JSON", rec.Code)
	}
}

func TestLogoutInvalidatesEveryToken(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "me@example.com", "supersecret", "member")
	phone := env.tokenFor(t, m.ID)
	laptop := env.tokenFor(t, m.ID)

	if rec := env.do(t, "GET", "/api/me", phone, nil); rec.Code != http.StatusOK {
		t.Fatalf("token should work before logout, got %d", rec.Code)
	}
	if rec := env.do(t, "POST", "/api/auth/logout", phone, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", rec.Code)
	}
	for name, tok := range map[string]string{"phone": phone, "laptop": laptop} {
		rec := env.do(t, "GET", "/api/me", tok, nil)
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "TOKEN_REVOKED" {
			t.Errorf("%s token after logout: status = %d, want 401 TOKEN_REVOKED", name, rec.Code)
		}
	}
	// login ใหม่ได้ตั๋วที่ใช้ได้
	rec := env.do(t, "POST", "/api/auth/login", "", map[string]string{"email": "me@example.com", "password": "supersecret"})
	if me := env.do(t, "GET", "/api/me", decode[sessionResp](t, rec).Token, nil); me.Code != http.StatusOK {
		t.Errorf("fresh login token rejected: %d", me.Code)
	}
}

func TestLogoutRequiresLogin(t *testing.T) {
	env := newTestEnv(t)
	if rec := env.do(t, "POST", "/api/auth/logout", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(env.members.revoked) != 0 {
		t.Error("RevokeTokens was called without a login")
	}
}
