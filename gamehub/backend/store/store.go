package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// error ของชั้น store handler ใช้ errors.Is แล้วแปลงเป็นรหัสตอบกลับ
var (
	ErrNotFound     = errors.New("not found")
	ErrDuplicate    = errors.New("duplicate")
	ErrEmailTaken   = errors.New("email taken")
	ErrNoGame       = errors.New("game not found")
	ErrBadReference = errors.New("bad reference") // อ้างถึงแนว/แพลตฟอร์ม/ผู้พัฒนาที่ไม่มีอยู่จริง
)

// รหัส error ของ PostgreSQL ที่เราจับ
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

type Store struct {
	Pool *pgxpool.Pool
}

// pgCode คืนรหัส error ของ Postgres ถ้า err มาจากฐานข้อมูล ไม่งั้นคืนค่าว่าง
func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// scanner ใช้ได้ทั้ง pgx.Row และ pgx.Rows
type scanner interface {
	Scan(dest ...any) error
}
