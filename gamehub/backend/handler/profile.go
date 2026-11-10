package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"gamehub/auth"
	"gamehub/mw"
	"gamehub/store"
)

const maxAvatarURLLen = 500

type updateProfileReq struct {
	DisplayName string  `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"` // null หรือ "" = ล้างรูป
}

type changePasswordReq struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// cleanAvatarURL คืน (ค่าที่จะเก็บ, ใช้ได้ไหม) รับเฉพาะลิงก์ http/https เท่านั้น
// กันค่าอย่าง javascript: ที่อาจถูกหน้าเว็บนำไปใช้เป็น src/href
func cleanAvatarURL(p *string) (*string, bool) {
	if p == nil {
		return nil, true
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil, true
	}
	if len(s) > maxAvatarURLLen {
		return nil, false
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	return &s, true
}

// Me ส่งข้อมูลของคนที่ login อยู่ (ไม่มีรหัสผ่านและ token_version ออกไปกับ JSON)
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	m, err := h.Members.MemberByID(r.Context(), mw.WhoFrom(r.Context()).ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "ไม่พบบัญชีนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// UpdateProfile แก้ชื่อที่แสดงและรูปประจำตัวของตัวเอง เลขสมาชิกมาจากตั๋วเท่านั้น
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileReq
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if !validName(name) {
		writeError(w, http.StatusBadRequest, "BAD_NAME", "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
		return
	}
	avatar, ok := cleanAvatarURL(req.AvatarURL)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_AVATAR", "รูปประจำตัวต้องเป็นลิงก์ http หรือ https ยาวไม่เกิน 500 ตัวอักษร")
		return
	}
	m, err := h.Members.UpdateProfile(r.Context(), mw.WhoFrom(r.Context()).ID, name, avatar)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "ไม่พบบัญชีนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// ChangePassword ต้องส่งรหัสผ่านเดิมมายืนยันก่อน เปลี่ยนสำเร็จแล้วตั๋วทุกใบที่เคยออกไปจะใช้ไม่ได้
// จึงออกตั๋วใบใหม่ให้เครื่องที่เปลี่ยนรหัส (เครื่องนี้ไม่ต้อง login ใหม่ แต่เครื่องอื่นต้อง)
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if writePasswordError(w, auth.ValidatePassword(req.NewPassword)) {
		return
	}
	id := mw.WhoFrom(r.Context()).ID

	current, err := h.Members.PasswordHashByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "ไม่พบบัญชีนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	// ใช้ 400 ไม่ใช่ 401 เพราะ 401 หน้าเว็บมักตีความว่าตั๋วหมดอายุแล้วเด้งไปหน้า login
	if !auth.CheckPassword(current, req.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "WRONG_PASSWORD", "รหัสผ่านปัจจุบันไม่ถูกต้อง")
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "SAME_PASSWORD", "รหัสผ่านใหม่ต้องไม่ซ้ำกับรหัสเดิม")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		internal(w, err)
		return
	}
	version, err := h.Members.ChangePassword(r.Context(), id, hash)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "ไม่พบบัญชีนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	token, err := h.Tickets.Issue(id, version)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
