// Package config อ่านค่าตั้งต้นจาก environment variable และตรวจให้ครบตั้งแต่เปิดโปรแกรม
// ถ้าค่าไหนขาดหรือผิด ให้ล้มทันทีพร้อมบอกครบทุกข้อ ดีกว่าไปพังตอนมีคนใช้งาน
package config

import (
	"errors"
	"os"
	"strings"
)

// MinSecretLen คือความยาวต่ำสุดของ JWT_SECRET (ตามต้นแบบ moviehub)
const MinSecretLen = 32

type Config struct {
	Port           string   // พอร์ตที่ฟัง เช่น "8080" (Render ตั้ง PORT ให้เอง)
	DatabaseURL    string   // connection string ของ Supabase
	JWTSecret      string   // กุญแจเซ็น token
	AllowedOrigins []string // origin ของหน้าเว็บที่อนุญาตให้เรียก API (CORS)
}

// Load อ่านค่าจาก env แล้วตรวจ ถ้ามีปัญหาจะคืน error ที่รวมทุกข้อไว้ด้วยกัน
func Load() (Config, error) {
	return load(os.Getenv)
}

// load แยกออกมาเพื่อให้เทสต์ส่งฟังก์ชันอ่านค่าของตัวเองเข้ามาได้
func load(get func(string) string) (Config, error) {
	cfg := Config{
		Port:           strings.TrimSpace(get("PORT")),
		DatabaseURL:    strings.TrimSpace(get("DATABASE_URL")),
		JWTSecret:      get("JWT_SECRET"),
		AllowedOrigins: splitList(get("ALLOWED_ORIGINS")),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	var problems []string
	if cfg.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL ว่างอยู่")
	}
	if len(cfg.JWTSecret) < MinSecretLen {
		problems = append(problems, "JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร")
	}
	if len(cfg.AllowedOrigins) == 0 {
		problems = append(problems, "ALLOWED_ORIGINS ว่างอยู่ (ใส่ origin ของหน้าเว็บ เช่น http://localhost:5173)")
	}
	if len(problems) > 0 {
		return Config{}, errors.New(strings.Join(problems, "; "))
	}
	return cfg, nil
}

// splitList แยกด้วยจุลภาค ตัดช่องว่าง และทิ้งค่าว่าง
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
