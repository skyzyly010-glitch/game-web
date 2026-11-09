package store

import "context"

type Rating struct {
	GameID int64 `json:"gameId"`
	Score  int16 `json:"score"`
}

// PutRating ให้คะแนนเกม ถ้าเคยให้แล้วจะแก้เป็นคะแนนใหม่ (upsert)
func (s *Store) PutRating(ctx context.Context, memberID, gameID int64, score int16) (Rating, error) {
	var r Rating
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO ratings (member_id, game_id, score)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (member_id, game_id)
		 DO UPDATE SET score = EXCLUDED.score, updated_at = now()
		 RETURNING game_id, score`, memberID, gameID, score).
		Scan(&r.GameID, &r.Score)
	if pgCode(err) == pgForeignKeyViolation {
		return Rating{}, ErrNoGame
	}
	return r, err
}

// DeleteRating ลบคะแนนของตัวเอง ถ้าไม่เคยให้ก็ถือว่าสำเร็จ (ผลลัพธ์เหมือนกัน)
func (s *Store) DeleteRating(ctx context.Context, memberID, gameID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM ratings WHERE member_id = $1 AND game_id = $2`, memberID, gameID)
	return err
}
