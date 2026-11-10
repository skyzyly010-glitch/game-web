package handler

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"gamehub/auth"
	"gamehub/mw"
	"gamehub/store"
)

const (
	maxEmailLen     = 255
	maxNameRunes    = 50
	badCredsMessage = "อีเมลหรือรหัสผ่านไม่ถูกต้อง"
)

type registerReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// normalizeEmail ทำให้อีเมลเป็นรูปเดียวกันทั้งตอนสมัครและตอน login (ตัดช่องว่าง ตัวพิมพ์เล็ก)
func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validEmail(s string) bool {
	if s == "" || len(s) > maxEmailLen {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s // ไม่รับรูปแบบ "ชื่อ <อีเมล>"
}

func validName(s string) bool {
	return s != "" && utf8.RuneCountInString(s) <= maxNameRunes
}

// writePasswordError แปลงผลตรวจรหัสผ่านเป็นคำตอบ ถ้าไม่มี error คืน false
func writePasswordError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, auth.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, "BAD_PASSWORD", "รหัสผ่านต้องยาวอย่างน้อย 8 ตัวอักษร")
	default:
		writeError(w, http.StatusBadRequest, "BAD_PASSWORD", "รหัสผ่านยาวเกินไป (ไม่เกิน 72 ไบต์)")
	}
	return true
}

// session ออกตั๋วให้สมาชิกแล้วตอบ {token, member}
func (h *Handler) session(w http.ResponseWriter, status int, m store.Member) {
	token, err := h.Tickets.Issue(m.ID, m.TokenVersion)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, status, map[string]any{"token": token, "member": m})
}

// Register สมัครสมาชิก role เป็น member เสมอ (ไม่รับ role จากผู้ใช้) และ login ให้เลยทันที
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decodeJSON(w, r, &req) {
		return
	}
	email := normalizeEmail(req.Email)
	name := strings.TrimSpace(req.DisplayName)
	switch {
	case !validEmail(email):
		writeError(w, http.StatusBadRequest, "BAD_EMAIL", "รูปแบบอีเมลไม่ถูกต้อง")
		return
	case !validName(name):
		writeError(w, http.StatusBadRequest, "BAD_NAME", "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
		return
	case writePasswordError(w, auth.ValidatePassword(req.Password)):
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internal(w, err)
		return
	}
	m, err := h.Members.CreateMember(r.Context(), email, hash, name)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	h.session(w, http.StatusCreated, m)
}

// Login ตอบข้อความเดียวกันทั้งกรณี "ไม่มีอีเมลนี้" และ "รหัสผิด"
// และใช้เวลาใกล้เคียงกัน คนภายนอกจึงใช้ login เดาไม่ได้ว่าอีเมลไหนมีบัญชี
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !decodeJSON(w, r, &req) {
		return
	}
	m, err := h.Members.MemberByEmail(r.Context(), normalizeEmail(req.Email))
	if errors.Is(err, store.ErrNotFound) {
		auth.SpendPasswordTime(req.Password)
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", badCredsMessage)
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	if !auth.CheckPassword(m.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", badCredsMessage)
		return
	}
	h.session(w, http.StatusOK, m)
}

// Logout ยกเลิกตั๋วทุกใบของสมาชิกคนนี้ (เพิ่ม token_version) ไม่ใช่แค่ใบที่ใช้อยู่
// จึงเป็นการออกจากระบบทุกเครื่อง ตั๋วเก่าที่หลุดไปก็ใช้ไม่ได้อีก
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	who := mw.WhoFrom(r.Context())
	if err := h.Members.RevokeTokens(r.Context(), who.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
