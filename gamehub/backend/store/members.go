package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Member struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	AvatarURL    *string   `json:"avatarUrl"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	TokenVersion int       `json:"-"` // ใช้เทียบกับตั๋ว ไม่ส่งออกไปกับ JSON
	PasswordHash string    `json:"-"` // ไม่มีวันออกไปกับ JSON
}

const memberCols = `id, email, display_name, avatar_url, role, created_at, token_version`

func scanMember(row scanner) (Member, error) {
	var m Member
	err := row.Scan(&m.ID, &m.Email, &m.DisplayName, &m.AvatarURL, &m.Role, &m.CreatedAt, &m.TokenVersion)
	return m, err
}

// CreateMember สร้างสมาชิกใหม่ role เป็น member เสมอ (ค่า default ของตาราง)
func (s *Store) CreateMember(ctx context.Context, email, hash, name string) (Member, error) {
	m := Member{Email: email, DisplayName: name}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO members (email, password_hash, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id, avatar_url, role, created_at, token_version`, email, hash, name).
		Scan(&m.ID, &m.AvatarURL, &m.Role, &m.CreatedAt, &m.TokenVersion)
	if pgCode(err) == pgUniqueViolation {
		return Member{}, ErrEmailTaken
	}
	return m, err
}

// MemberByEmail ใช้ตอน login (email ต้อง lower-case มาแล้ว) คืนรหัสผ่านที่เข้ารหัสมาด้วย
func (s *Store) MemberByEmail(ctx context.Context, email string) (Member, error) {
	var m Member
	err := s.Pool.QueryRow(ctx,
		`SELECT `+memberCols+`, password_hash FROM members WHERE lower(email) = $1`, email).
		Scan(&m.ID, &m.Email, &m.DisplayName, &m.AvatarURL, &m.Role, &m.CreatedAt, &m.TokenVersion, &m.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

// MemberByID ใช้ใน middleware ทุกครั้งที่ตรวจตั๋ว เพื่อเทียบ token_version และ role ล่าสุด
func (s *Store) MemberByID(ctx context.Context, id int64) (Member, error) {
	m, err := scanMember(s.Pool.QueryRow(ctx,
		`SELECT `+memberCols+` FROM members WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

// PasswordHashByID ใช้ตรวจ "รหัสผ่านเดิม" ก่อนเปลี่ยนรหัสผ่าน
func (s *Store) PasswordHashByID(ctx context.Context, id int64) (string, error) {
	var hash string
	err := s.Pool.QueryRow(ctx, `SELECT password_hash FROM members WHERE id = $1`, id).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

func (s *Store) ListMembers(ctx context.Context) ([]Member, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+memberCols+` FROM members ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Member{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateProfile แทนที่ชื่อที่แสดงและรูปประจำตัว (avatar เป็น nil = ล้างรูป)
func (s *Store) UpdateProfile(ctx context.Context, id int64, displayName string, avatarURL *string) (Member, error) {
	m, err := scanMember(s.Pool.QueryRow(ctx,
		`UPDATE members SET display_name = $2, avatar_url = $3, updated_at = now()
		 WHERE id = $1
		 RETURNING `+memberCols, id, displayName, avatarURL))
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

// ChangePassword เปลี่ยนรหัสผ่านและเพิ่ม token_version ในคำสั่งเดียว
// ตั๋วเก่าทุกใบจึงใช้ไม่ได้ทันที คืนเลขเวอร์ชันใหม่ไว้ออกตั๋วใบใหม่ให้เครื่องที่เปลี่ยนรหัส
func (s *Store) ChangePassword(ctx context.Context, id int64, newHash string) (int, error) {
	var version int
	err := s.Pool.QueryRow(ctx,
		`UPDATE members
		 SET password_hash = $2, token_version = token_version + 1, updated_at = now()
		 WHERE id = $1
		 RETURNING token_version`, id, newHash).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return version, err
}

// RevokeTokens ใช้ตอน logout เพิ่ม token_version ทำให้ตั๋วเดิมทุกใบใช้ไม่ได้
func (s *Store) RevokeTokens(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE members SET token_version = token_version + 1 WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetRole ให้ admin เปลี่ยนบทบาท พร้อมเพิ่ม token_version เพื่อให้ตั๋วที่มี role เก่าใช้ไม่ได้
func (s *Store) SetRole(ctx context.Context, id int64, role string) (Member, error) {
	m, err := scanMember(s.Pool.QueryRow(ctx,
		`UPDATE members SET role = $2, token_version = token_version + 1, updated_at = now()
		 WHERE id = $1
		 RETURNING `+memberCols, id, role))
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

// UpsertAdmin ใช้โดย cmd/seed: สร้างบัญชี admin ถ้ายังไม่มี ถ้ามีอีเมลนี้อยู่แล้วจะตั้งรหัสผ่านใหม่
// และยกเป็น admin พร้อมเพิ่ม token_version (email ต้อง lower-case มาแล้ว)
func (s *Store) UpsertAdmin(ctx context.Context, email, hash, name string) (Member, error) {
	m, err := scanMember(s.Pool.QueryRow(ctx,
		`INSERT INTO members (email, password_hash, display_name, role)
		 VALUES ($1, $2, $3, 'admin')
		 ON CONFLICT (lower(email))
		 DO UPDATE SET password_hash = EXCLUDED.password_hash, role = 'admin',
		               token_version = members.token_version + 1, updated_at = now()
		 RETURNING `+memberCols, email, hash, name))
	return m, err
}

// CountAdmins ให้ handler กันไม่ให้ลบหรือลดสิทธิ์ admin คนสุดท้าย
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM members WHERE role = 'admin'`).Scan(&n)
	return n, err
}

// DeleteMember ลบสมาชิก (รีวิว คะแนน รายการเกมของคนนั้นถูกลบตาม ON DELETE CASCADE)
func (s *Store) DeleteMember(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM members WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
