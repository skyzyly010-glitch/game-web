// Package handler รับ HTTP request ตรวจข้อมูลที่ส่งมา เรียก store แล้วตอบเป็น JSON
// ไม่มี SQL อยู่ที่นี่ (อยู่ใน store) และไม่ตรวจตั๋วเอง (อยู่ใน mw)
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"gamehub/auth"
	"gamehub/store"
)

// MemberStore คือสิ่งที่ handler หมวดสมาชิกต้องการจากชั้น store
// ประกาศเป็น interface เพื่อให้เทสต์ด้วย store ปลอมได้โดยไม่ต้องมีฐานข้อมูล
type MemberStore interface {
	CreateMember(ctx context.Context, email, hash, name string) (store.Member, error)
	MemberByEmail(ctx context.Context, email string) (store.Member, error)
	MemberByID(ctx context.Context, id int64) (store.Member, error)
	PasswordHashByID(ctx context.Context, id int64) (string, error)
	UpdateProfile(ctx context.Context, id int64, displayName string, avatarURL *string) (store.Member, error)
	ChangePassword(ctx context.Context, id int64, newHash string) (int, error)
	RevokeTokens(ctx context.Context, id int64) error
}

var _ MemberStore = (*store.Store)(nil) // ถ้า store เปลี่ยนจนไม่ตรง จะ build ไม่ผ่านตรงนี้

type Handler struct {
	Members MemberStore
	Tickets *auth.Issuer
}

// ขนาดตัว request ที่รับได้สูงสุด กันคนส่งข้อมูลก้อนใหญ่มาเกาะหน่วยความจำ
const maxBodyBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง
func internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

// decodeJSON อ่าน body เป็น JSON ถ้าอ่านไม่ได้จะตอบ error ให้เองแล้วคืน false
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "ข้อมูลที่ส่งมาใหญ่เกินไป")
			return false
		}
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านข้อมูล JSON ไม่ออก")
		return false
	}
	return true
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// Health ใช้ให้ Render เช็กว่าเซิร์ฟเวอร์ยังทำงานอยู่ (ไม่แตะฐานข้อมูล)
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
