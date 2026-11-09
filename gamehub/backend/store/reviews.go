package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReviewAuthor struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
}

type Review struct {
	ID        int64        `json:"id"`
	MemberID  int64        `json:"memberId"`
	GameID    int64        `json:"gameId"`
	Text      string       `json:"text"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	Member    ReviewAuthor `json:"member"`
	Score     *int16       `json:"score"` // คะแนนที่คนเขียนรีวิวให้เกมนี้ (ถ้ามี)
}

// ListReviews คืนรีวิวของเกมหนึ่งเรื่อง ใหม่สุดก่อน
func (s *Store) ListReviews(ctx context.Context, gameID int64) ([]Review, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT r.id, r.member_id, r.game_id, r.text, r.created_at, r.updated_at,
		        m.display_name, rt.score
		 FROM reviews r
		 JOIN members m ON m.id = r.member_id
		 LEFT JOIN ratings rt ON rt.member_id = r.member_id AND rt.game_id = r.game_id
		 WHERE r.game_id = $1
		 ORDER BY r.created_at DESC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.MemberID, &r.GameID, &r.Text,
			&r.CreatedAt, &r.UpdatedAt, &r.Member.DisplayName, &r.Score); err != nil {
			return nil, err
		}
		r.Member.ID = r.MemberID
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddReview เพิ่มรีวิว 1 คนต่อ 1 เกม ซ้ำคืน ErrDuplicate ไม่มีเกมคืน ErrNoGame
func (s *Store) AddReview(ctx context.Context, memberID, gameID int64, text string) (Review, error) {
	r := Review{MemberID: memberID, GameID: gameID, Text: text}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO reviews (member_id, game_id, text)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`, memberID, gameID, text).
		Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	switch pgCode(err) {
	case pgUniqueViolation:
		return Review{}, ErrDuplicate
	case pgForeignKeyViolation:
		return Review{}, ErrNoGame
	}
	return r, err
}

// UpdateReview แก้ข้อความรีวิว เฉพาะเจ้าของเท่านั้น (admin ลบได้แต่ไม่แก้คำพูดของคนอื่น)
// ไม่พบรีวิว หรือไม่ใช่ของคนนี้ คืน ErrNotFound เหมือนกัน เพื่อไม่บอกว่ามีรีวิวนี้อยู่
func (s *Store) UpdateReview(ctx context.Context, reviewID, memberID int64, text string) (Review, error) {
	r := Review{MemberID: memberID, Text: text}
	err := s.Pool.QueryRow(ctx,
		`UPDATE reviews SET text = $3, updated_at = now()
		 WHERE id = $1 AND member_id = $2
		 RETURNING id, game_id, created_at, updated_at`, reviewID, memberID, text).
		Scan(&r.ID, &r.GameID, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	return r, err
}

// DeleteReview ลบรีวิว เจ้าของลบของตัวเอง หรือ admin ลบได้ทุกอัน
func (s *Store) DeleteReview(ctx context.Context, reviewID, memberID int64, isAdmin bool) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM reviews WHERE id = $1 AND (member_id = $2 OR $3::boolean)`,
		reviewID, memberID, isAdmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MyReview คือรีวิวของตัวเอง พร้อมชื่อเกม ใช้ในหน้า profile
type MyReview struct {
	ID        int64     `json:"id"`
	GameID    int64     `json:"gameId"`
	GameTitle string    `json:"gameTitle"`
	GameSlug  string    `json:"gameSlug"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *Store) ListMyReviews(ctx context.Context, memberID int64) ([]MyReview, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT r.id, r.game_id, g.title, g.slug, r.text, r.created_at, r.updated_at
		 FROM reviews r
		 JOIN games g ON g.id = r.game_id
		 WHERE r.member_id = $1
		 ORDER BY r.created_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []MyReview{}
	for rows.Next() {
		var r MyReview
		if err := rows.Scan(&r.ID, &r.GameID, &r.GameTitle, &r.GameSlug,
			&r.Text, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
