package auth

import (
	"errors"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLen   = 8
	MaxPasswordBytes = 72 // bcrypt อ่านแค่ 72 ไบต์แรก ที่เกินจะถูกมองข้ามเงียบ ๆ จึงไม่รับ
)

var (
	ErrPasswordTooShort = errors.New("password too short")
	ErrPasswordTooLong  = errors.New("password too long")
)

// ValidatePassword ตรวจความยาวก่อนเข้ารหัส
// ขั้นต่ำนับเป็นตัวอักษร (ไม่ให้รหัสไทย 3 ตัวผ่านเพราะมี 9 ไบต์) ส่วนขั้นสูงนับเป็นไบต์ตามข้อจำกัดของ bcrypt
func ValidatePassword(plain string) error {
	if utf8.RuneCountInString(plain) < MinPasswordLen {
		return ErrPasswordTooShort
	}
	if len(plain) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword แปลงรหัสผ่านเป็นลายนิ้วมือ bcrypt ที่ย้อนกลับไม่ได้
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword ตอบว่ารหัสที่พิมพ์มาตรงกับลายนิ้วมือที่เก็บไว้ไหม
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

var (
	dummyOnce sync.Once
	dummyHash []byte
)

// SpendPasswordTime ใช้ตอน login ที่ไม่พบอีเมลนี้ในระบบ: รัน bcrypt กับลายนิ้วมือปลอมหนึ่งครั้ง
// ให้เวลาตอบใกล้เคียงกับกรณีที่พบอีเมลแต่รหัสผิด คนภายนอกจึงใช้เวลาตอบเดาไม่ได้ว่ามีอีเมลนี้หรือไม่
func SpendPasswordTime(plain string) {
	dummyOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(plain))
}
