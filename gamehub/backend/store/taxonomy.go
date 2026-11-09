package store

import "context"

// Option คือรายการให้เลือกในตัวกรองและฟอร์มของ admin (แนว, แพลตฟอร์ม, ผู้พัฒนา)
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (s *Store) ListGenres(ctx context.Context) ([]Option, error) {
	return s.options(ctx, `SELECT id, name FROM genres ORDER BY name`)
}

func (s *Store) ListPlatforms(ctx context.Context) ([]Option, error) {
	return s.options(ctx, `SELECT id, name FROM platforms ORDER BY name`)
}

func (s *Store) ListCompanies(ctx context.Context) ([]Option, error) {
	return s.options(ctx, `SELECT id, name FROM companies ORDER BY name`)
}

// options รับเฉพาะคำสั่งคงที่ที่เราเขียนไว้ด้านบน ห้ามส่งข้อความจากผู้ใช้เข้ามา
func (s *Store) options(ctx context.Context, query string) ([]Option, error) {
	rows, err := s.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Option{}
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
