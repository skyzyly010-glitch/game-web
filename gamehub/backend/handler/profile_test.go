package handler

import (
	"net/http"
	"strings"
	"testing"

	"gamehub/auth"
	"gamehub/store"
)

func TestProfileRoutesRequireLogin(t *testing.T) {
	env := newTestEnv(t)
	for _, r := range [][2]string{{"GET", "/api/me"}, {"PUT", "/api/me"}, {"PUT", "/api/me/password"}} {
		if rec := env.do(t, r[0], r[1], "", map[string]string{}); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: status = %d, want 401", r[0], r[1], rec.Code)
		}
	}
}

func TestMe(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "me@example.com", "supersecret", "admin")
	rec := env.do(t, "GET", "/api/me", env.tokenFor(t, m.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := decode[store.Member](t, rec)
	if got.Email != "me@example.com" || got.Role != "admin" {
		t.Errorf("member = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "$2a$") || strings.Contains(rec.Body.String(), "tokenVersion") {
		t.Errorf("response leaks internal fields: %s", rec.Body)
	}
}

func TestMeForDeletedMember(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "gone@example.com", "supersecret", "member")
	tok := env.tokenFor(t, m.ID)
	delete(env.members.byID, m.ID)
	if rec := env.do(t, "GET", "/api/me", tok, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a member who no longer exists", rec.Code)
	}
}

func TestUpdateProfile(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "me@example.com", "supersecret", "member")
	tok := env.tokenFor(t, m.ID)

	rec := env.do(t, "PUT", "/api/me", tok, map[string]any{
		"displayName": "  New Name ", "avatarUrl": "https://example.com/a.png",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", rec.Code, rec.Body)
	}
	got := decode[store.Member](t, rec)
	if got.DisplayName != "New Name" || got.AvatarURL == nil || *got.AvatarURL != "https://example.com/a.png" {
		t.Errorf("member = %+v", got)
	}

	// ส่ง avatarUrl เป็น null = ล้างรูป
	rec = env.do(t, "PUT", "/api/me", tok, map[string]any{"displayName": "New Name", "avatarUrl": nil})
	if rec.Code != http.StatusOK || decode[store.Member](t, rec).AvatarURL != nil {
		t.Errorf("avatar not cleared: status %d body %s", rec.Code, rec.Body)
	}
}

func TestUpdateProfileOnlyAffectsCallerAndIgnoresIDFromBody(t *testing.T) {
	env := newTestEnv(t)
	me := env.members.add(t, "me@example.com", "supersecret", "member")
	other := env.members.add(t, "other@example.com", "supersecret", "member")
	rec := env.do(t, "PUT", "/api/me", env.tokenFor(t, me.ID), map[string]any{
		"displayName": "Hacked", "id": other.ID, "role": "admin",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if env.members.byID[other.ID].DisplayName != "Tester" {
		t.Error("another member's profile was modified")
	}
	if env.members.byID[me.ID].Role != "member" {
		t.Error("role was changed through the profile endpoint")
	}
}

func TestUpdateProfileValidation(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "me@example.com", "supersecret", "member")
	tok := env.tokenFor(t, m.ID)
	tests := []struct {
		name string
		body map[string]any
		want string
	}{
		{"empty name", map[string]any{"displayName": " "}, "BAD_NAME"},
		{"name too long", map[string]any{"displayName": strings.Repeat("x", 51)}, "BAD_NAME"},
		{"javascript avatar", map[string]any{"displayName": "A", "avatarUrl": "javascript:alert(1)"}, "BAD_AVATAR"},
		{"javascript avatar with host", map[string]any{"displayName": "A", "avatarUrl": "javascript://example.com/%0Aalert(1)"}, "BAD_AVATAR"},
		{"ftp avatar", map[string]any{"displayName": "A", "avatarUrl": "ftp://example.com/a.png"}, "BAD_AVATAR"},
		{"data avatar", map[string]any{"displayName": "A", "avatarUrl": "data:text/html;base64,AAAA"}, "BAD_AVATAR"},
		{"relative avatar", map[string]any{"displayName": "A", "avatarUrl": "/img/a.png"}, "BAD_AVATAR"},
		{"avatar too long", map[string]any{"displayName": "A", "avatarUrl": "https://example.com/" + strings.Repeat("a", 500)}, "BAD_AVATAR"},
	}
	for _, tt := range tests {
		rec := env.do(t, "PUT", "/api/me", tok, tt.body)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != tt.want {
			t.Errorf("%s: status = %d code = %q, want 400 %s", tt.name, rec.Code, errCode(t, rec), tt.want)
		}
	}
}

func TestChangePasswordSuccess(t *testing.T) {
	env := newTestEnv(t)
	m := env.members.add(t, "me@example.com", "old-password", "member")
	thisDevice := env.tokenFor(t, m.ID)
	otherDevice := env.tokenFor(t, m.ID)

	rec := env.do(t, "PUT", "/api/me/password", thisDevice, map[string]string{
		"currentPassword": "old-password", "newPassword": "brand-new-pass",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", rec.Code, rec.Body)
	}
	newToken := decode[map[string]string](t, rec)["token"]
	if newToken == "" {
		t.Fatal("no replacement token returned")
	}

	stored := env.members.byID[m.ID].PasswordHash
	if !auth.CheckPassword(stored, "brand-new-pass") || auth.CheckPassword(stored, "old-password") {
		t.Error("stored password was not replaced by the new one")
	}
	// เครื่องอื่น (ตั๋วเก่า) ต้องถูกเตะออก ส่วนเครื่องนี้ใช้ตั๋วใหม่ต่อได้
	if rec := env.do(t, "GET", "/api/me", otherDevice, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("old token still works after password change: %d", rec.Code)
	}
	if rec := env.do(t, "GET", "/api/me", thisDevice, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("the token used for the change should also be replaced: %d", rec.Code)
	}
	if rec := env.do(t, "GET", "/api/me", newToken, nil); rec.Code != http.StatusOK {
		t.Errorf("replacement token rejected: %d", rec.Code)
	}
	// login ด้วยรหัสใหม่ได้ ด้วยรหัสเก่าไม่ได้
	if rec := env.do(t, "POST", "/api/auth/login", "", map[string]string{"email": "me@example.com", "password": "brand-new-pass"}); rec.Code != http.StatusOK {
		t.Errorf("login with new password failed: %d", rec.Code)
	}
	if rec := env.do(t, "POST", "/api/auth/login", "", map[string]string{"email": "me@example.com", "password": "old-password"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("login with old password should fail: %d", rec.Code)
	}
}

func TestChangePasswordRejections(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		newPass    string
		wantStatus int
		wantCode   string
	}{
		{"wrong current password", "not-my-password", "brand-new-pass", http.StatusBadRequest, "WRONG_PASSWORD"},
		{"new password too short", "old-password", "short", http.StatusBadRequest, "BAD_PASSWORD"},
		{"new password too long", "old-password", strings.Repeat("a", 73), http.StatusBadRequest, "BAD_PASSWORD"},
		{"same as current", "old-password", "old-password", http.StatusBadRequest, "SAME_PASSWORD"},
	}
	for _, tt := range tests {
		env := newTestEnv(t)
		m := env.members.add(t, "me@example.com", "old-password", "member")
		before := env.members.byID[m.ID].PasswordHash
		rec := env.do(t, "PUT", "/api/me/password", env.tokenFor(t, m.ID), map[string]string{
			"currentPassword": tt.current, "newPassword": tt.newPass,
		})
		if rec.Code != tt.wantStatus || errCode(t, rec) != tt.wantCode {
			t.Errorf("%s: status = %d code = %q, want %d %s", tt.name, rec.Code, errCode(t, rec), tt.wantStatus, tt.wantCode)
		}
		if env.members.byID[m.ID].PasswordHash != before || env.members.byID[m.ID].TokenVersion != 0 {
			t.Errorf("%s: password or token version changed despite rejection", tt.name)
		}
	}
}
