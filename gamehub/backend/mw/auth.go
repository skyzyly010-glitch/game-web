// Package mw รวม middleware: ตรวจตั๋ว ตรวจสิทธิ์ CORS และ log
package mw

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"gamehub/auth"
)

// Who คือผู้เรียกที่ผ่านการตรวจแล้ว ใส่ไว้ใน context ให้ handler ดึงไปใช้
// handler ต้องใช้ ID จากตรงนี้เท่านั้น ห้ามรับเลขสมาชิกจาก body หรือ URL
type Who struct {
	ID   int64
	Role string
}

type ctxKey struct{}

func WhoFrom(ctx context.Context) Who {
	w, _ := ctx.Value(ctxKey{}).(Who)
	return w
}

// MemberState คือสถานะล่าสุดของสมาชิกในฐานข้อมูล
type MemberState struct {
	Role         string
	TokenVersion int
}

// ErrNoMember ให้ Lookup คืนเมื่อไม่พบสมาชิก (เช่นถูกลบไปแล้ว)
var ErrNoMember = errors.New("mw: member not found")

// Lookup ถามฐานข้อมูลว่าสมาชิกคนนี้ตอนนี้มี role และ token_version อะไร
// ส่งเป็นฟังก์ชันเพื่อให้ mw ไม่ต้องรู้จักฐานข้อมูล และเทสต์ได้โดยไม่ต้องมี DB
type Lookup func(ctx context.Context, memberID int64) (MemberState, error)

func deny(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

// RequireLogin คือประตูตรวจตั๋ว ทำ 3 ขั้น:
//  1. ตั๋วต้องถูกต้อง (ลายเซ็น ไม่หมดอายุ)
//  2. สมาชิกต้องยังอยู่ในระบบ
//  3. token_version ในตั๋วต้องเท่ากับของจริงในฐานข้อมูล ถ้าไม่เท่า (logout หรือเปลี่ยนรหัสผ่านไปแล้ว) ตั๋วใบนั้นใช้ไม่ได้ทันที
//
// role ที่ handler เห็นมาจากฐานข้อมูลตอนนี้ ไม่ใช่จากตั๋ว
func RequireLogin(iss *auth.Issuer, lookup Lookup, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found || token == "" {
			deny(w, http.StatusUnauthorized, "LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อน")
			return
		}
		id, version, err := iss.Parse(token)
		if err != nil {
			deny(w, http.StatusUnauthorized, "TOKEN_INVALID", "ตั๋วใช้ไม่ได้หรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		state, err := lookup(r.Context(), id)
		if errors.Is(err, ErrNoMember) {
			deny(w, http.StatusUnauthorized, "TOKEN_INVALID", "ตั๋วใช้ไม่ได้หรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		if err != nil {
			log.Printf("auth lookup failed: %v", err)
			deny(w, http.StatusInternalServerError, "INTERNAL", "เกิดข้อผิดพลาดในระบบ กรุณาลองใหม่")
			return
		}
		if state.TokenVersion != version {
			deny(w, http.StatusUnauthorized, "TOKEN_REVOKED", "เซสชันนี้สิ้นสุดแล้ว กรุณาเข้าสู่ระบบใหม่")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, Who{ID: id, Role: state.Role})
		next(w, r.WithContext(ctx))
	}
}

// RequireRole ใช้ครอบหลัง RequireLogin เท่านั้น: ถ้า role ไม่ตรง ตอบ 403
// ตรวจที่ฝั่งเซิร์ฟเวอร์เสมอ ไม่พึ่งว่าหน้าเว็บซ่อนปุ่มไว้
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if WhoFrom(r.Context()).Role != role {
			deny(w, http.StatusForbidden, "FORBIDDEN", "บัญชีนี้ไม่มีสิทธิ์ทำรายการนี้")
			return
		}
		next(w, r)
	}
}
