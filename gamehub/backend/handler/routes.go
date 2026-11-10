package handler

import (
	"context"
	"errors"
	"net/http"

	"gamehub/mw"
	"gamehub/store"
)

// MemberLookup ต่อ store เข้ากับ mw.RequireLogin (แปลง "ไม่พบสมาชิก" เป็น error ที่ mw เข้าใจ)
func MemberLookup(s MemberStore) mw.Lookup {
	return func(ctx context.Context, id int64) (mw.MemberState, error) {
		m, err := s.MemberByID(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			return mw.MemberState{}, mw.ErrNoMember
		}
		if err != nil {
			return mw.MemberState{}, err
		}
		return mw.MemberState{Role: m.Role, TokenVersion: m.TokenVersion}, nil
	}
}

// Routes ลงทะเบียนเส้นทางทั้งหมดของ API ว่าเส้นทางไหนต้อง login หรือเป็น admin ดูได้ที่ไฟล์นี้ที่เดียว
func (h *Handler) Routes(mux *http.ServeMux, lookup mw.Lookup) {
	login := func(next http.HandlerFunc) http.HandlerFunc {
		return mw.RequireLogin(h.Tickets, lookup, next)
	}

	mux.HandleFunc("GET /api/health", h.Health)

	// ใครก็เรียกได้ (guest)
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)

	// ต้อง login (member และ admin)
	mux.HandleFunc("POST /api/auth/logout", login(h.Logout))
	mux.HandleFunc("GET /api/me", login(h.Me))
	mux.HandleFunc("PUT /api/me", login(h.UpdateProfile))
	mux.HandleFunc("PUT /api/me/password", login(h.ChangePassword))
}
