// Package auth ออกและตรวจตั๋ว (JWT) และจัดการรหัสผ่าน (bcrypt)
package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MinSecretLen ตรงกับที่ config บังคับ (JWT_SECRET ต้องยาวอย่างน้อย 32 ตัว)
const MinSecretLen = 32

// Claims คือสิ่งที่อยู่ในตั๋ว มีแค่เลขสมาชิก (Subject) กับ token_version
// ไม่ใส่ role ไว้ในตั๋ว เพราะ role ที่ฝังไว้จะค้างอยู่จนตั๋วหมดอายุแม้ admin ถูกลดสิทธิ์ไปแล้ว
// middleware จะอ่าน role ล่าสุดจากฐานข้อมูลแทน
type Claims struct {
	Version int `json:"tv"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

// NewIssuer ปฏิเสธกุญแจสั้นหรืออายุตั๋วที่ไม่เป็นบวก เพื่อไม่ให้ระบบเริ่มทำงานแบบไม่ปลอดภัย
func NewIssuer(secret []byte, ttl time.Duration) (*Issuer, error) {
	if len(secret) < MinSecretLen {
		return nil, errors.New("auth: JWT secret สั้นเกินไป")
	}
	if ttl <= 0 {
		return nil, errors.New("auth: ttl ต้องมากกว่าศูนย์")
	}
	return &Issuer{secret: secret, ttl: ttl}, nil
}

// Issue ออกตั๋วให้สมาชิกหนึ่งคน โดยผูกกับ token_version ปัจจุบันของเขา
func (i *Issuer) Issue(memberID int64, version int) (string, error) {
	now := time.Now()
	claims := Claims{
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(memberID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

// Parse ตรวจลายเซ็น วิธีเซ็น (ล็อกที่ HS256) และเวลาหมดอายุ (บังคับให้มี)
// ผ่านแล้วคืนเลขสมาชิกกับ token_version ในตั๋ว ซึ่งผู้เรียกต้องเทียบกับค่าในฐานข้อมูลต่อ
func (i *Issuer) Parse(tokenString string) (memberID int64, version int, err error) {
	var c Claims
	_, err = jwt.ParseWithClaims(tokenString, &c,
		func(t *jwt.Token) (any, error) { return i.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return 0, 0, err
	}
	memberID, err = strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || memberID <= 0 {
		return 0, 0, errors.New("auth: subject ของตั๋วไม่ถูกต้อง")
	}
	return memberID, c.Version, nil
}
