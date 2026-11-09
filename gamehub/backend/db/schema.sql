-- GameHub database schema (PostgreSQL 14+ / Supabase)
-- รันไฟล์นี้ก่อน แล้วค่อยรัน seed.sql
-- ตัวอย่างเกมใน seed.sql เป็นข้อมูลตัวอย่างที่เขียนเอง ไม่ได้ดึงจาก IGDB

BEGIN;

-- ---------------------------------------------------------------------------
-- genres: แนวเกม (Action, RPG, ...)
-- ---------------------------------------------------------------------------
CREATE TABLE genres (
    id    SERIAL       PRIMARY KEY,
    name  VARCHAR(50)  NOT NULL UNIQUE
);

-- ---------------------------------------------------------------------------
-- platforms: เครื่องที่เล่นได้ (PC, PS5, Switch, ...)
-- ---------------------------------------------------------------------------
CREATE TABLE platforms (
    id    SERIAL       PRIMARY KEY,
    name  VARCHAR(50)  NOT NULL UNIQUE
);

-- ---------------------------------------------------------------------------
-- companies: ผู้พัฒนา (developer) 1 เกมมี 1 ผู้พัฒนาหลัก
-- ---------------------------------------------------------------------------
CREATE TABLE companies (
    id    SERIAL        PRIMARY KEY,
    name  VARCHAR(100)  NOT NULL UNIQUE
);

-- ---------------------------------------------------------------------------
-- games: เกม 1 แถวต่อ 1 เรื่อง
-- slug ใช้ทำลิงก์ที่อ่านง่าย เช่น /games/elden-ring
-- ---------------------------------------------------------------------------
CREATE TABLE games (
    id            BIGSERIAL     PRIMARY KEY,
    title         VARCHAR(255)  NOT NULL CHECK (char_length(btrim(title)) >= 1),
    slug          VARCHAR(255)  NOT NULL UNIQUE,
    summary       TEXT,
    cover_url     VARCHAR(500),                  -- ว่างได้ ให้หน้าเว็บแสดงภาพแทน
    release_date  DATE,
    company_id    INTEGER       REFERENCES companies (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_games_release_date ON games (release_date DESC);
CREATE INDEX idx_games_title_lower  ON games (lower(title));
CREATE INDEX idx_games_company      ON games (company_id);

-- ---------------------------------------------------------------------------
-- game_genres: เกม 1 เรื่องมีได้หลายแนว
-- position = 0 คือแนวหลักที่หน้ารายการแสดง
-- ---------------------------------------------------------------------------
CREATE TABLE game_genres (
    game_id   BIGINT    NOT NULL REFERENCES games (id)  ON DELETE CASCADE,
    genre_id  INTEGER   NOT NULL REFERENCES genres (id),
    position  SMALLINT  NOT NULL CHECK (position >= 0),
    PRIMARY KEY (game_id, genre_id),
    UNIQUE (game_id, position)
);

CREATE INDEX idx_game_genres_genre ON game_genres (genre_id);

-- ---------------------------------------------------------------------------
-- game_platforms: เกม 1 เรื่องออกได้หลายเครื่อง
-- ---------------------------------------------------------------------------
CREATE TABLE game_platforms (
    game_id      BIGINT   NOT NULL REFERENCES games (id)     ON DELETE CASCADE,
    platform_id  INTEGER  NOT NULL REFERENCES platforms (id),
    PRIMARY KEY (game_id, platform_id)
);

CREATE INDEX idx_game_platforms_platform ON game_platforms (platform_id);

-- ===========================================================================
-- ข้อมูลสมาชิกและกิจกรรมของสมาชิก
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- members: สมาชิกของเว็บ
-- token_version: เพิ่มค่าทุกครั้งที่ logout หรือเปลี่ยนรหัสผ่าน
--                token เก่าที่เลขไม่ตรงจะใช้ไม่ได้ทันที
-- ---------------------------------------------------------------------------
CREATE TABLE members (
    id             BIGSERIAL     PRIMARY KEY,
    email          VARCHAR(255)  NOT NULL,
    password_hash  VARCHAR(255)  NOT NULL,       -- bcrypt เท่านั้น ห้ามเก็บรหัสผ่านตรง ๆ
    display_name   VARCHAR(50)   NOT NULL CHECK (char_length(btrim(display_name)) >= 1),
    avatar_url     VARCHAR(500),
    role           VARCHAR(10)   NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    token_version  INTEGER       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- อีเมลห้ามซ้ำโดยไม่สนตัวพิมพ์เล็กใหญ่ (Go ต้อง lower() ก่อนบันทึกและก่อนค้นตอน login)
CREATE UNIQUE INDEX uq_members_email ON members (lower(email));

-- ---------------------------------------------------------------------------
-- ratings: คะแนนที่สมาชิกให้เกม 1-10
-- 1 คน 1 เกม 1 คะแนน ให้ใหม่ = แก้คะแนนเดิม (upsert)
-- ---------------------------------------------------------------------------
CREATE TABLE ratings (
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id     BIGINT       NOT NULL REFERENCES games (id)   ON DELETE CASCADE,
    score       SMALLINT     NOT NULL CHECK (score BETWEEN 1 AND 10),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, game_id)
);

CREATE INDEX idx_ratings_game ON ratings (game_id);

-- คะแนนเฉลี่ยต่อเกม คำนวณสดจากตาราง ratings ไม่ต้องเก็บซ้ำ
CREATE VIEW game_rating_stats AS
SELECT game_id,
       count(*)              AS rating_count,
       round(avg(score), 1)  AS rating_average
FROM ratings
GROUP BY game_id;

-- ---------------------------------------------------------------------------
-- reviews: รีวิวของสมาชิก 1 คนต่อ 1 เกม (แก้ไขได้)
-- ---------------------------------------------------------------------------
CREATE TABLE reviews (
    id          BIGSERIAL    PRIMARY KEY,
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id     BIGINT       NOT NULL REFERENCES games (id)   ON DELETE CASCADE,
    text        TEXT         NOT NULL CHECK (char_length(btrim(text)) >= 10),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (member_id, game_id)
);

CREATE INDEX idx_reviews_game_created ON reviews (game_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- member_games: รายการเกมของสมาชิก พร้อมสถานะ
--   want    = อยากเล่น
--   playing = กำลังเล่น
--   played  = เล่นจบแล้ว
--   backlog = ซื้อไว้ยังไม่ได้เล่น
-- 1 คน 1 เกม 1 สถานะ เปลี่ยนสถานะ = แก้แถวเดิม (upsert)
-- ---------------------------------------------------------------------------
CREATE TABLE member_games (
    member_id  BIGINT        NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id    BIGINT        NOT NULL REFERENCES games (id)   ON DELETE CASCADE,
    status     VARCHAR(10)   NOT NULL CHECK (status IN ('want', 'playing', 'played', 'backlog')),
    note       VARCHAR(200),
    added_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, game_id)
);

CREATE INDEX idx_member_games_member_status ON member_games (member_id, status, added_at DESC);

COMMIT;
