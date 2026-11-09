package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// GameSummary คือข้อมูลที่หน้ารายการเกมใช้
type GameSummary struct {
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	Slug          string   `json:"slug"`
	CoverURL      *string  `json:"coverUrl"`
	Year          *int     `json:"year"`
	Genre         *string  `json:"genre"` // แนวหลัก (position = 0)
	RatingAverage *float64 `json:"ratingAverage"`
	RatingCount   int      `json:"ratingCount"`
}

// GameDetail คือข้อมูลเต็มของหน้ารายละเอียดเกม
type GameDetail struct {
	GameSummary
	Summary     *string  `json:"summary"`
	ReleaseDate *string  `json:"releaseDate"` // รูปแบบ YYYY-MM-DD
	Developer   *string  `json:"developer"`
	Genres      []string `json:"genres"`
	Platforms   []string `json:"platforms"`
}

// GameFilter คือเงื่อนไขค้นหารายการเกม
type GameFilter struct {
	Query      string
	GenreID    int
	PlatformID int
	Sort       string // newest | rating | title
	Limit      int
	Offset     int
}

// ลำดับการเรียงที่อนุญาต ค่าจากผู้ใช้ใช้แค่เป็นกุญแจเลือกในแผนที่นี้
// ไม่เคยถูกต่อเข้าไปในคำสั่ง SQL โดยตรง
var gameOrder = map[string]string{
	"newest": "g.release_date DESC NULLS LAST, g.id DESC",
	"rating": "s.rating_average DESC NULLS LAST, s.rating_count DESC NULLS LAST, g.id DESC",
	"title":  "lower(g.title) ASC, g.id ASC",
}

func ValidGameSort(sort string) bool {
	_, ok := gameOrder[sort]
	return ok
}

// เครื่องหมาย % _ \ ในคำค้นต้องถูกมองเป็นตัวอักษรธรรมดา ไม่ใช่ wildcard ของ LIKE
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func clampPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ListGames คืนรายการเกมหนึ่งหน้า กับจำนวนเกมทั้งหมดที่ตรงเงื่อนไข (ไว้ทำปุ่มเปลี่ยนหน้า)
// ค่าจากผู้ใช้เดินทางผ่าน $1 $2 ... แยกจากคำสั่งเสมอ
func (s *Store) ListGames(ctx context.Context, f GameFilter) ([]GameSummary, int, error) {
	limit, offset := clampPage(f.Limit, f.Offset)

	var (
		conds []string
		args  []any
	)
	// add เปลี่ยน ? ในเงื่อนไขเป็น $n ตามลำดับพารามิเตอร์
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		add(`lower(g.title) LIKE '%' || lower(?) || '%' ESCAPE '\'`, likeEscaper.Replace(q))
	}
	if f.GenreID > 0 {
		add(`EXISTS (SELECT 1 FROM game_genres gg WHERE gg.game_id = g.id AND gg.genre_id = ?)`, f.GenreID)
	}
	if f.PlatformID > 0 {
		add(`EXISTS (SELECT 1 FROM game_platforms gp WHERE gp.game_id = g.id AND gp.platform_id = ?)`, f.PlatformID)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM games g`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order, ok := gameOrder[f.Sort]
	if !ok {
		order = gameOrder["newest"]
	}
	args = append(args, limit, offset)
	query := `SELECT g.id, g.title, g.slug, g.cover_url,
	                 extract(year FROM g.release_date)::int,
	                 (SELECT ge.name FROM game_genres gg
	                    JOIN genres ge ON ge.id = gg.genre_id
	                   WHERE gg.game_id = g.id AND gg.position = 0),
	                 s.rating_average::float8,
	                 coalesce(s.rating_count, 0)::int
	          FROM games g
	          LEFT JOIN game_rating_stats s ON s.game_id = g.id` + where +
		` ORDER BY ` + order +
		` LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []GameSummary{}
	for rows.Next() {
		var g GameSummary
		if err := rows.Scan(&g.ID, &g.Title, &g.Slug, &g.CoverURL, &g.Year,
			&g.Genre, &g.RatingAverage, &g.RatingCount); err != nil {
			return nil, 0, err
		}
		items = append(items, g)
	}
	return items, total, rows.Err()
}

// GameByID คืนข้อมูลเต็มของเกมหนึ่งเรื่อง ไม่พบคืน ErrNotFound
func (s *Store) GameByID(ctx context.Context, id int64) (GameDetail, error) {
	var g GameDetail
	err := s.Pool.QueryRow(ctx,
		`SELECT g.id, g.title, g.slug, g.cover_url,
		        extract(year FROM g.release_date)::int,
		        (SELECT ge.name FROM game_genres gg
		           JOIN genres ge ON ge.id = gg.genre_id
		          WHERE gg.game_id = g.id AND gg.position = 0),
		        s.rating_average::float8,
		        coalesce(s.rating_count, 0)::int,
		        g.summary, to_char(g.release_date, 'YYYY-MM-DD'), c.name
		 FROM games g
		 LEFT JOIN companies c ON c.id = g.company_id
		 LEFT JOIN game_rating_stats s ON s.game_id = g.id
		 WHERE g.id = $1`, id).
		Scan(&g.ID, &g.Title, &g.Slug, &g.CoverURL, &g.Year, &g.Genre,
			&g.RatingAverage, &g.RatingCount, &g.Summary, &g.ReleaseDate, &g.Developer)
	if errors.Is(err, pgx.ErrNoRows) {
		return GameDetail{}, ErrNotFound
	}
	if err != nil {
		return GameDetail{}, err
	}

	if g.Genres, err = s.names(ctx,
		`SELECT ge.name FROM game_genres gg
		   JOIN genres ge ON ge.id = gg.genre_id
		  WHERE gg.game_id = $1 ORDER BY gg.position`, id); err != nil {
		return GameDetail{}, err
	}
	if g.Platforms, err = s.names(ctx,
		`SELECT p.name FROM game_platforms gp
		   JOIN platforms p ON p.id = gp.platform_id
		  WHERE gp.game_id = $1 ORDER BY p.name`, id); err != nil {
		return GameDetail{}, err
	}
	return g, nil
}

// names รันคำสั่งที่คืนคอลัมน์ข้อความเดียว ได้ผลเป็น slice ที่ไม่เป็น nil เสมอ
func (s *Store) names(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// ส่วนของ admin: เพิ่ม แก้ ลบเกม
// ---------------------------------------------------------------------------

// GameInput คือข้อมูลที่ admin ส่งมาตอนเพิ่มหรือแก้เกม (handler ตรวจรูปแบบมาแล้ว)
type GameInput struct {
	Title       string
	Slug        string
	Summary     *string
	CoverURL    *string
	ReleaseDate *time.Time
	CompanyID   *int
	GenreIDs    []int // ลำดับแรกคือแนวหลัก
	PlatformIDs []int
}

func (s *Store) CreateGame(ctx context.Context, in GameInput) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) // ไม่มีผลถ้า Commit ไปแล้ว

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO games (title, slug, summary, cover_url, release_date, company_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		in.Title, in.Slug, in.Summary, in.CoverURL, in.ReleaseDate, in.CompanyID).Scan(&id)
	if err != nil {
		return 0, mapGameErr(err)
	}
	if err := setGameLinks(ctx, tx, id, in.GenreIDs, in.PlatformIDs); err != nil {
		return 0, mapGameErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) UpdateGame(ctx context.Context, id int64, in GameInput) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE games
		 SET title = $2, slug = $3, summary = $4, cover_url = $5,
		     release_date = $6, company_id = $7, updated_at = now()
		 WHERE id = $1`,
		id, in.Title, in.Slug, in.Summary, in.CoverURL, in.ReleaseDate, in.CompanyID)
	if err != nil {
		return mapGameErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := setGameLinks(ctx, tx, id, in.GenreIDs, in.PlatformIDs); err != nil {
		return mapGameErr(err)
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteGame(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM games WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// setGameLinks แทนที่แนวและแพลตฟอร์มของเกมทั้งชุดใน transaction เดียวกับตัวเกม
// position ของแนวคือลำดับที่ส่งมา (ตัวแรก = 0 = แนวหลัก)
func setGameLinks(ctx context.Context, tx pgx.Tx, gameID int64, genreIDs, platformIDs []int) error {
	genreIDs = uniqueInts(genreIDs)
	platformIDs = uniqueInts(platformIDs)

	if _, err := tx.Exec(ctx, `DELETE FROM game_genres WHERE game_id = $1`, gameID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM game_platforms WHERE game_id = $1`, gameID); err != nil {
		return err
	}
	if len(genreIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO game_genres (game_id, genre_id, position)
			 SELECT $1, t.genre_id, (t.ord - 1)::smallint
			 FROM unnest($2::int[]) WITH ORDINALITY AS t(genre_id, ord)`,
			gameID, genreIDs); err != nil {
			return err
		}
	}
	if len(platformIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO game_platforms (game_id, platform_id)
			 SELECT $1, p FROM unnest($2::int[]) AS p`,
			gameID, platformIDs); err != nil {
			return err
		}
	}
	return nil
}

func mapGameErr(err error) error {
	switch pgCode(err) {
	case pgUniqueViolation:
		return ErrDuplicate // slug ซ้ำ
	case pgForeignKeyViolation:
		return ErrBadReference // แนว/แพลตฟอร์ม/ผู้พัฒนาไม่มีจริง
	}
	return err
}

// uniqueInts ตัดตัวซ้ำโดยคงลำดับเดิมไว้
func uniqueInts(in []int) []int {
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
