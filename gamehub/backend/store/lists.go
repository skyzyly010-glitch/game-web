package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// สถานะที่อนุญาตในรายการเกมของสมาชิก (ตรงกับ CHECK ในตาราง member_games)
//
//	want = อยากเล่น, playing = กำลังเล่น, played = เล่นจบแล้ว, backlog = ซื้อไว้ยังไม่ได้เล่น
var validStatuses = map[string]struct{}{
	"want": {}, "playing": {}, "played": {}, "backlog": {},
}

func ValidStatus(s string) bool {
	_, ok := validStatuses[s]
	return ok
}

type MemberGame struct {
	GameID   int64     `json:"gameId"`
	Title    string    `json:"title"`
	Slug     string    `json:"slug"`
	CoverURL *string   `json:"coverUrl"`
	Status   string    `json:"status"`
	Note     *string   `json:"note"`
	AddedAt  time.Time `json:"addedAt"`
}

// ListMemberGames คืนรายการเกมของสมาชิก status ว่างคือเอาทุกสถานะ
func (s *Store) ListMemberGames(ctx context.Context, memberID int64, status string) ([]MemberGame, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT mg.game_id, g.title, g.slug, g.cover_url, mg.status, mg.note, mg.added_at
		 FROM member_games mg
		 JOIN games g ON g.id = mg.game_id
		 WHERE mg.member_id = $1 AND ($2::text = '' OR mg.status = $2::text)
		 ORDER BY mg.added_at DESC`, memberID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []MemberGame{}
	for rows.Next() {
		var m MemberGame
		if err := rows.Scan(&m.GameID, &m.Title, &m.Slug, &m.CoverURL, &m.Status, &m.Note, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PutMemberGame เพิ่มเกมเข้ารายการหรือเปลี่ยนสถานะ (upsert) note เป็น nil คือล้างโน้ต
// ถ้าไม่มีเกมนี้ คำสั่ง INSERT...SELECT จะไม่ได้แถวเลย จึงคืน ErrNoGame
func (s *Store) PutMemberGame(ctx context.Context, memberID, gameID int64, status string, note *string) (MemberGame, error) {
	var m MemberGame
	err := s.Pool.QueryRow(ctx,
		`WITH saved AS (
		   INSERT INTO member_games (member_id, game_id, status, note)
		   SELECT $1::bigint, g.id, $3::varchar, $4::varchar FROM games g WHERE g.id = $2
		   ON CONFLICT (member_id, game_id)
		   DO UPDATE SET status = EXCLUDED.status, note = EXCLUDED.note, updated_at = now()
		   RETURNING game_id, status, note, added_at
		 )
		 SELECT saved.game_id, g.title, g.slug, g.cover_url, saved.status, saved.note, saved.added_at
		 FROM saved JOIN games g ON g.id = saved.game_id`,
		memberID, gameID, status, note).
		Scan(&m.GameID, &m.Title, &m.Slug, &m.CoverURL, &m.Status, &m.Note, &m.AddedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MemberGame{}, ErrNoGame
	}
	return m, err
}

func (s *Store) DeleteMemberGame(ctx context.Context, memberID, gameID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM member_games WHERE member_id = $1 AND game_id = $2`, memberID, gameID)
	return err
}

// MyGameState คือสิ่งที่สมาชิกคนนี้ทำกับเกมนี้ไว้ ใช้แสดงบนหน้ารายละเอียดเกมในครั้งเดียว
type MyGameState struct {
	Rating *int16  `json:"rating"` // ยังไม่ให้คะแนน = null
	Status *string `json:"status"` // ยังไม่อยู่ในรายการ = null
	Note   *string `json:"note"`
}

func (s *Store) MyGameState(ctx context.Context, memberID, gameID int64) (MyGameState, error) {
	var st MyGameState
	err := s.Pool.QueryRow(ctx,
		`SELECT r.score, mg.status, mg.note
		 FROM games g
		 LEFT JOIN ratings r      ON r.game_id = g.id  AND r.member_id = $1
		 LEFT JOIN member_games mg ON mg.game_id = g.id AND mg.member_id = $1
		 WHERE g.id = $2`, memberID, gameID).
		Scan(&st.Rating, &st.Status, &st.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return MyGameState{}, ErrNoGame
	}
	return st, err
}
