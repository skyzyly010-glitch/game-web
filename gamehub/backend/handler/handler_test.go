package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gamehub/auth"
	"gamehub/store"
)

// fakeMembers คือ store สมาชิกในหน่วยความจำ ทำงานเหมือนของจริงในส่วนที่ handler พึ่งพา
type fakeMembers struct {
	byID    map[int64]*store.Member
	nextID  int64
	revoked []int64 // เลขสมาชิกที่ถูกเรียก RevokeTokens
	failAll error   // ตั้งค่าเพื่อจำลองฐานข้อมูลล่ม
}

func newFakeMembers() *fakeMembers {
	return &fakeMembers{byID: map[int64]*store.Member{}, nextID: 1}
}

// add สร้างสมาชิกตรง ๆ ในหน่วยความจำ (ข้ามการตรวจของ handler) ใช้เตรียมข้อมูลเทสต์
func (f *fakeMembers) add(t *testing.T, email, password, role string) store.Member {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	m := &store.Member{
		ID: f.nextID, Email: email, DisplayName: "Tester", Role: role,
		PasswordHash: hash, CreatedAt: time.Now(),
	}
	f.byID[m.ID] = m
	f.nextID++
	return *m
}

func (f *fakeMembers) CreateMember(_ context.Context, email, hash, name string) (store.Member, error) {
	if f.failAll != nil {
		return store.Member{}, f.failAll
	}
	for _, m := range f.byID {
		if strings.EqualFold(m.Email, email) {
			return store.Member{}, store.ErrEmailTaken
		}
	}
	m := &store.Member{ID: f.nextID, Email: email, DisplayName: name, Role: "member",
		PasswordHash: hash, CreatedAt: time.Now()}
	f.byID[m.ID] = m
	f.nextID++
	return *m, nil
}

func (f *fakeMembers) MemberByEmail(_ context.Context, email string) (store.Member, error) {
	if f.failAll != nil {
		return store.Member{}, f.failAll
	}
	for _, m := range f.byID {
		if strings.EqualFold(m.Email, email) {
			return *m, nil
		}
	}
	return store.Member{}, store.ErrNotFound
}

func (f *fakeMembers) MemberByID(_ context.Context, id int64) (store.Member, error) {
	if f.failAll != nil {
		return store.Member{}, f.failAll
	}
	if m, ok := f.byID[id]; ok {
		return *m, nil
	}
	return store.Member{}, store.ErrNotFound
}

func (f *fakeMembers) PasswordHashByID(_ context.Context, id int64) (string, error) {
	m, err := f.MemberByID(context.Background(), id)
	return m.PasswordHash, err
}

func (f *fakeMembers) UpdateProfile(_ context.Context, id int64, name string, avatar *string) (store.Member, error) {
	m, ok := f.byID[id]
	if !ok {
		return store.Member{}, store.ErrNotFound
	}
	m.DisplayName, m.AvatarURL = name, avatar
	return *m, nil
}

func (f *fakeMembers) ChangePassword(_ context.Context, id int64, newHash string) (int, error) {
	m, ok := f.byID[id]
	if !ok {
		return 0, store.ErrNotFound
	}
	m.PasswordHash = newHash
	m.TokenVersion++
	return m.TokenVersion, nil
}

func (f *fakeMembers) RevokeTokens(_ context.Context, id int64) error {
	m, ok := f.byID[id]
	if !ok {
		return store.ErrNotFound
	}
	m.TokenVersion++
	f.revoked = append(f.revoked, id)
	return nil
}

// testEnv คือเซิร์ฟเวอร์จำลองที่ต่อเส้นทางจริง (รวม RequireLogin) เข้ากับ store ปลอม
type testEnv struct {
	srv     http.Handler
	members *fakeMembers
	tickets *auth.Issuer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tickets, err := auth.NewIssuer([]byte("a secret key with at least 32 bytes long"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	members := newFakeMembers()
	h := &Handler{Members: members, Tickets: tickets}
	mux := http.NewServeMux()
	h.Routes(mux, MemberLookup(members))
	return &testEnv{srv: mux, members: members, tickets: tickets}
}

// do ยิง request เข้าเซิร์ฟเวอร์จำลอง body เป็น nil, string (ส่งตรง ๆ) หรือค่าที่จะแปลงเป็น JSON
func (e *testEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		if err := json.NewEncoder(&buf).Encode(b); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	return rec
}

// tokenFor ออกตั๋วที่ตรงกับสถานะปัจจุบันของสมาชิก
func (e *testEnv) tokenFor(t *testing.T, id int64) string {
	t.Helper()
	tok, err := e.tickets.Issue(id, e.members.byID[id].TokenVersion)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("cannot decode response %q: %v", rec.Body.String(), err)
	}
	return v
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	type body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	b := decode[body](t, rec)
	if b.Error.Message == "" {
		t.Errorf("error message is empty in %q", rec.Body.String())
	}
	return b.Error.Code
}

func TestHealth(t *testing.T) {
	rec := newTestEnv(t).do(t, "GET", "/api/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
