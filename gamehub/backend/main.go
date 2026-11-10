// GameHub API: ต่อสายทุกชั้นเข้าด้วยกันแล้วเปิดเซิร์ฟเวอร์
// ลำดับคือ config -> ฐานข้อมูล -> store -> handler -> middleware -> เซิร์ฟเวอร์
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gamehub/auth"
	"gamehub/config"
	"gamehub/handler"
	"gamehub/mw"
	"gamehub/store"
)

const tokenTTL = time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("ตั้งค่าไม่ครบหรือไม่ถูกต้อง: %v", err)
	}

	// รับสัญญาณปิด (Ctrl+C หรือที่ Render ส่งตอนรีสตาร์ต) เพื่อปิดเซิร์ฟเวอร์อย่างเรียบร้อย
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DATABASE_URL อ่านไม่ออก (ไม่แสดงค่าเพื่อความปลอดภัย)")
	}
	// Supabase pooler (โหมด transaction) ไม่รองรับ prepared statement ที่ pgx เก็บไว้ใช้ซ้ำ
	// โหมดนี้ส่งคำสั่งแบบไม่เก็บ จึงใช้ได้ทั้งต่อตรงและผ่าน pooler
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	poolCfg.MaxConns = 5 // แพ็กเกจฟรีของ Supabase จำกัดจำนวนการเชื่อมต่อ

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalf("สร้าง connection pool ไม่สำเร็จ: %v", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("ต่อฐานข้อมูลไม่ได้: %v", err)
	}

	tickets, err := auth.NewIssuer([]byte(cfg.JWTSecret), tokenTTL)
	if err != nil {
		log.Fatalf("ตั้งค่าตั๋วไม่สำเร็จ: %v", err)
	}

	st := &store.Store{Pool: pool}
	h := &handler.Handler{Members: st, Tickets: tickets}

	mux := http.NewServeMux()
	h.Routes(mux, handler.MemberLookup(st))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mw.Logging(mw.CORS(cfg.AllowedOrigins, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("ปิดเซิร์ฟเวอร์ไม่เรียบร้อย: %v", err)
		}
	}()

	log.Printf("GameHub API ฟังที่พอร์ต %s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("เซิร์ฟเวอร์หยุดทำงาน: %v", err)
	}
	log.Print("ปิดเซิร์ฟเวอร์แล้ว")
}