package mw

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gamehub/auth"
)

func testIssuer(t *testing.T) *auth.Issuer {
	t.Helper()
	iss, err := auth.NewIssuer([]byte("a secret key with at least 32 bytes long"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return iss
}

// lookupOf สร้าง Lookup ปลอมที่รู้จักสมาชิก id 1 ตามสถานะที่กำหนด
func lookupOf(state MemberState, err error) Lookup {
	return func(_ context.Context, id int64) (MemberState, error) {
		if err != nil {
			return MemberState{}, err
		}
		if id != 1 {
			return MemberState{}, ErrNoMember
		}
		return state, nil
	}
}

// call ยิง request ผ่าน RequireLogin และคืนผล พร้อมบอกว่า handler ปลายทางถูกเรียกไหม
func call(t *testing.T, iss *auth.Issuer, lookup Lookup, header string) (rec *httptest.ResponseRecorder, reached bool, who Who) {
	t.Helper()
	h := RequireLogin(iss, lookup, func(w http.ResponseWriter, r *http.Request) {
		reached = true
		who = WhoFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec = httptest.NewRecorder()
	h(rec, req)
	return rec, reached, who
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the expected error JSON: %v (%q)", err, rec.Body.String())
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
	return body.Error.Code
}

func TestRequireLoginRejectsMissingOrMalformedHeader(t *testing.T) {
	iss := testIssuer(t)
	for _, header := range []string{"", "Bearer ", "Basic abc", "token-without-prefix"} {
		rec, reached, _ := call(t, iss, lookupOf(MemberState{Role: "member"}, nil), header)
		if rec.Code != http.StatusUnauthorized || reached {
			t.Errorf("header %q: status=%d reached=%v, want 401 and not reached", header, rec.Code, reached)
		}
		if got := errorCode(t, rec); got != "LOGIN_REQUIRED" {
			t.Errorf("header %q: code = %q, want LOGIN_REQUIRED", header, got)
		}
	}
}

func TestRequireLoginRejectsGarbageToken(t *testing.T) {
	rec, reached, _ := call(t, testIssuer(t), lookupOf(MemberState{Role: "member"}, nil), "Bearer not.a.jwt")
	if rec.Code != http.StatusUnauthorized || reached {
		t.Fatalf("status=%d reached=%v, want 401 and not reached", rec.Code, reached)
	}
	if got := errorCode(t, rec); got != "TOKEN_INVALID" {
		t.Errorf("code = %q, want TOKEN_INVALID", got)
	}
}

func TestRequireLoginAcceptsValidToken(t *testing.T) {
	iss := testIssuer(t)
	token, _ := iss.Issue(1, 3)
	rec, reached, who := call(t, iss, lookupOf(MemberState{Role: "member", TokenVersion: 3}, nil), "Bearer "+token)
	if rec.Code != http.StatusNoContent || !reached {
		t.Fatalf("status=%d reached=%v, want handler reached", rec.Code, reached)
	}
	if who.ID != 1 || who.Role != "member" {
		t.Errorf("Who = %+v, want {1 member}", who)
	}
}

func TestRequireLoginRejectsRevokedToken(t *testing.T) {
	iss := testIssuer(t)
	token, _ := iss.Issue(1, 3) // ตั๋วออกตอน version 3
	// ต่อมา logout หรือเปลี่ยนรหัสผ่าน ฐานข้อมูลเลื่อนเป็น version 4
	rec, reached, _ := call(t, iss, lookupOf(MemberState{Role: "member", TokenVersion: 4}, nil), "Bearer "+token)
	if rec.Code != http.StatusUnauthorized || reached {
		t.Fatalf("status=%d reached=%v, want 401 and not reached", rec.Code, reached)
	}
	if got := errorCode(t, rec); got != "TOKEN_REVOKED" {
		t.Errorf("code = %q, want TOKEN_REVOKED", got)
	}
}

func TestRequireLoginRejectsDeletedMember(t *testing.T) {
	iss := testIssuer(t)
	token, _ := iss.Issue(99, 0) // ไม่มีสมาชิก id 99 ในฐานข้อมูล
	rec, reached, _ := call(t, iss, lookupOf(MemberState{Role: "member"}, nil), "Bearer "+token)
	if rec.Code != http.StatusUnauthorized || reached {
		t.Fatalf("status=%d reached=%v, want 401 and not reached", rec.Code, reached)
	}
}

func TestRequireLoginReturns500WhenLookupFails(t *testing.T) {
	iss := testIssuer(t)
	token, _ := iss.Issue(1, 0)
	rec, reached, _ := call(t, iss, lookupOf(MemberState{}, errors.New("db down")), "Bearer "+token)
	if rec.Code != http.StatusInternalServerError || reached {
		t.Fatalf("status=%d reached=%v, want 500 and not reached", rec.Code, reached)
	}
	if got := errorCode(t, rec); got != "INTERNAL" {
		t.Errorf("code = %q, want INTERNAL", got)
	}
}

// role ต้องมาจากฐานข้อมูลตอนนี้ ตั๋วเก่าของ admin ที่ถูกลดสิทธิ์แล้ว (และ version ถูกเลื่อนตาม) ใช้ไม่ได้
// ส่วนกรณีลดสิทธิ์โดยไม่เลื่อน version ก็ยังเห็น role ใหม่ทันที
func TestRequireLoginUsesLiveRoleFromDatabase(t *testing.T) {
	iss := testIssuer(t)
	token, _ := iss.Issue(1, 0)
	_, reached, who := call(t, iss, lookupOf(MemberState{Role: "member", TokenVersion: 0}, nil), "Bearer "+token)
	if !reached || who.Role != "member" {
		t.Fatalf("reached=%v role=%q, want role taken from database", reached, who.Role)
	}
}

func TestRequireRole(t *testing.T) {
	reached := false
	h := RequireRole("admin", func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})

	for _, tt := range []struct {
		name    string
		ctx     context.Context
		want    int
		reached bool
	}{
		{"member is forbidden", context.WithValue(context.Background(), ctxKey{}, Who{ID: 1, Role: "member"}), http.StatusForbidden, false},
		{"admin passes", context.WithValue(context.Background(), ctxKey{}, Who{ID: 2, Role: "admin"}), http.StatusNoContent, true},
		{"no identity is forbidden", context.Background(), http.StatusForbidden, false},
	} {
		reached = false
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/api/admin/members", nil).WithContext(tt.ctx))
		if rec.Code != tt.want || reached != tt.reached {
			t.Errorf("%s: status=%d reached=%v, want %d/%v", tt.name, rec.Code, reached, tt.want, tt.reached)
		}
		if tt.want == http.StatusForbidden {
			if got := errorCode(t, rec); got != "FORBIDDEN" {
				t.Errorf("%s: code = %q, want FORBIDDEN", tt.name, got)
			}
		}
	}
}
