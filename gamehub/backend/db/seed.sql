-- GameHub seed data (รันหลัง schema.sql)
-- ข้อมูลตัวอย่าง: ชื่อเกม วันวางขาย ผู้พัฒนา เป็นข้อเท็จจริงทั่วไป ส่วนเรื่องย่อเขียนขึ้นเอง
-- บัญชี admin ไม่ได้อยู่ในไฟล์นี้ เพราะต้องมี bcrypt hash ให้สร้างผ่าน cmd/seed

BEGIN;

-- ---------------------------------------------------------------------------
-- genres
-- ---------------------------------------------------------------------------
INSERT INTO genres (name) VALUES
    ('Action'),
    ('Adventure'),
    ('RPG'),
    ('Strategy'),
    ('Simulation'),
    ('Puzzle'),
    ('Platformer'),
    ('Shooter'),
    ('Racing'),
    ('Sports'),
    ('Fighting'),
    ('Horror'),
    ('Indie'),
    ('Sandbox');

-- ---------------------------------------------------------------------------
-- platforms
-- ---------------------------------------------------------------------------
INSERT INTO platforms (name) VALUES
    ('PC'),
    ('PlayStation 4'),
    ('PlayStation 5'),
    ('Xbox One'),
    ('Xbox Series X|S'),
    ('Nintendo Switch'),
    ('Wii U');

-- ---------------------------------------------------------------------------
-- companies
-- ---------------------------------------------------------------------------
INSERT INTO companies (name) VALUES
    ('Nintendo'),
    ('FromSoftware'),
    ('CD Projekt Red'),
    ('Supergiant Games'),
    ('Maddy Makes Games'),
    ('ConcernedApe'),
    ('Mojang Studios'),
    ('Valve'),
    ('Rockstar Games'),
    ('Team Cherry'),
    ('Firaxis Games'),
    ('Playground Games'),
    ('Psyonix'),
    ('Capcom'),
    ('Larian Studios'),
    ('ZA/UM');

-- ---------------------------------------------------------------------------
-- games (20 เรื่อง) cover_url เว้นว่างไว้ ให้หน้าเว็บแสดงภาพแทนไปก่อน
-- ---------------------------------------------------------------------------
INSERT INTO games (title, slug, summary, release_date, company_id) VALUES
    ('The Legend of Zelda: Breath of the Wild', 'zelda-breath-of-the-wild',
     'Open-world adventure where you explore a ruined kingdom, climb almost anything and solve problems with physics and your own ideas.',
     '2017-03-03', (SELECT id FROM companies WHERE name = 'Nintendo')),
    ('Elden Ring', 'elden-ring',
     'Large open-world action RPG with demanding combat, built around exploration and choosing your own route through the world.',
     '2022-02-25', (SELECT id FROM companies WHERE name = 'FromSoftware')),
    ('The Witcher 3: Wild Hunt', 'the-witcher-3',
     'Story-driven RPG about a monster hunter searching for a missing person, with branching quests across a big open world.',
     '2015-05-19', (SELECT id FROM companies WHERE name = 'CD Projekt Red')),
    ('Hades', 'hades',
     'Fast action roguelike where you fight out of the underworld, with a story that moves forward every time you fail.',
     '2020-09-17', (SELECT id FROM companies WHERE name = 'Supergiant Games')),
    ('Celeste', 'celeste',
     'Precision platformer about climbing a mountain, with tight controls, a generous assist mode and a story about anxiety.',
     '2018-01-25', (SELECT id FROM companies WHERE name = 'Maddy Makes Games')),
    ('Stardew Valley', 'stardew-valley',
     'Relaxed farming simulation where you rebuild a farm, make friends in town, fish, mine and slowly settle in.',
     '2016-02-26', (SELECT id FROM companies WHERE name = 'ConcernedApe')),
    ('Minecraft', 'minecraft',
     'Sandbox game about gathering blocks, building anything you imagine and surviving the night, alone or with friends.',
     '2011-11-18', (SELECT id FROM companies WHERE name = 'Mojang Studios')),
    ('Portal 2', 'portal-2',
     'First-person puzzle game built around a portal gun, with sharp humor and a cooperative mode for two players.',
     '2011-04-19', (SELECT id FROM companies WHERE name = 'Valve')),
    ('Half-Life 2', 'half-life-2',
     'Story-focused first-person shooter that mixes combat, physics puzzles and a tense, occupied future city.',
     '2004-11-16', (SELECT id FROM companies WHERE name = 'Valve')),
    ('Dark Souls', 'dark-souls',
     'Punishing action RPG known for careful combat, interconnected level design and a world explained through hints.',
     '2011-09-22', (SELECT id FROM companies WHERE name = 'FromSoftware')),
    ('Super Mario Odyssey', 'super-mario-odyssey',
     'Colorful 3D platformer where Mario borrows the abilities of creatures and objects while travelling between kingdoms.',
     '2017-10-27', (SELECT id FROM companies WHERE name = 'Nintendo')),
    ('Red Dead Redemption 2', 'red-dead-redemption-2',
     'Western open-world game about an outlaw gang falling apart, with a very detailed world and a slow, character-led story.',
     '2018-10-26', (SELECT id FROM companies WHERE name = 'Rockstar Games')),
    ('Hollow Knight', 'hollow-knight',
     'Hand-drawn action platformer set in a vast underground insect kingdom, with free exploration and hard bosses.',
     '2017-02-24', (SELECT id FROM companies WHERE name = 'Team Cherry')),
    ('Sid Meier''s Civilization VI', 'civilization-vi',
     'Turn-based strategy game where you build an empire from the ancient era to the future and win by several different routes.',
     '2016-10-21', (SELECT id FROM companies WHERE name = 'Firaxis Games')),
    ('Forza Horizon 5', 'forza-horizon-5',
     'Open-world racing festival set in Mexico, with a huge list of cars and events you can tackle in any order.',
     '2021-11-09', (SELECT id FROM companies WHERE name = 'Playground Games')),
    ('Rocket League', 'rocket-league',
     'Soccer played with rocket-powered cars, easy to pick up and very deep in the air, in short online matches.',
     '2015-07-07', (SELECT id FROM companies WHERE name = 'Psyonix')),
    ('Resident Evil 4', 'resident-evil-4',
     'Remake of the survival horror classic, following a special agent sent to rescue a kidnapped daughter in rural Europe.',
     '2023-03-24', (SELECT id FROM companies WHERE name = 'Capcom')),
    ('Street Fighter 6', 'street-fighter-6',
     'One-on-one fighting game with a single-player world tour mode and control options for both newcomers and veterans.',
     '2023-06-02', (SELECT id FROM companies WHERE name = 'Capcom')),
    ('Baldur''s Gate 3', 'baldurs-gate-3',
     'Party-based RPG with turn-based combat, where almost every choice and dialogue can change how the story unfolds.',
     '2023-08-03', (SELECT id FROM companies WHERE name = 'Larian Studios')),
    ('Disco Elysium', 'disco-elysium',
     'Detective RPG with no combat, where you investigate a murder through conversations and your character''s own inner voices.',
     '2019-10-15', (SELECT id FROM companies WHERE name = 'ZA/UM'));

-- ---------------------------------------------------------------------------
-- game_genres (position 0 = แนวหลัก)
-- ---------------------------------------------------------------------------
INSERT INTO game_genres (game_id, genre_id, position)
SELECT g.id, ge.id, v.pos
FROM (VALUES
    ('zelda-breath-of-the-wild', 'Adventure',  0),
    ('zelda-breath-of-the-wild', 'RPG',        1),
    ('elden-ring',               'RPG',        0),
    ('elden-ring',               'Adventure',  1),
    ('the-witcher-3',            'RPG',        0),
    ('hades',                    'Action',     0),
    ('hades',                    'Indie',      1),
    ('celeste',                  'Platformer', 0),
    ('celeste',                  'Indie',      1),
    ('stardew-valley',           'Simulation', 0),
    ('stardew-valley',           'RPG',        1),
    ('stardew-valley',           'Indie',      2),
    ('minecraft',                'Sandbox',    0),
    ('minecraft',                'Adventure',  1),
    ('portal-2',                 'Puzzle',     0),
    ('portal-2',                 'Adventure',  1),
    ('half-life-2',              'Shooter',    0),
    ('dark-souls',               'RPG',        0),
    ('dark-souls',               'Action',     1),
    ('super-mario-odyssey',      'Platformer', 0),
    ('super-mario-odyssey',      'Adventure',  1),
    ('red-dead-redemption-2',    'Action',     0),
    ('red-dead-redemption-2',    'Adventure',  1),
    ('hollow-knight',            'Platformer', 0),
    ('hollow-knight',            'Action',     1),
    ('hollow-knight',            'Indie',      2),
    ('civilization-vi',          'Strategy',   0),
    ('forza-horizon-5',          'Racing',     0),
    ('rocket-league',            'Sports',     0),
    ('rocket-league',            'Racing',     1),
    ('resident-evil-4',          'Horror',     0),
    ('resident-evil-4',          'Shooter',    1),
    ('street-fighter-6',         'Fighting',   0),
    ('baldurs-gate-3',           'RPG',        0),
    ('baldurs-gate-3',           'Strategy',   1),
    ('disco-elysium',            'RPG',        0),
    ('disco-elysium',            'Indie',      1)
) AS v (slug, genre, pos)
JOIN games  g  ON g.slug  = v.slug
JOIN genres ge ON ge.name = v.genre;

-- ---------------------------------------------------------------------------
-- game_platforms
-- ---------------------------------------------------------------------------
INSERT INTO game_platforms (game_id, platform_id)
SELECT g.id, p.id
FROM (VALUES
    ('zelda-breath-of-the-wild', 'Nintendo Switch'),
    ('zelda-breath-of-the-wild', 'Wii U'),
    ('elden-ring', 'PC'),
    ('elden-ring', 'PlayStation 4'),
    ('elden-ring', 'PlayStation 5'),
    ('elden-ring', 'Xbox One'),
    ('elden-ring', 'Xbox Series X|S'),
    ('the-witcher-3', 'PC'),
    ('the-witcher-3', 'PlayStation 4'),
    ('the-witcher-3', 'PlayStation 5'),
    ('the-witcher-3', 'Xbox One'),
    ('the-witcher-3', 'Xbox Series X|S'),
    ('the-witcher-3', 'Nintendo Switch'),
    ('hades', 'PC'),
    ('hades', 'Nintendo Switch'),
    ('hades', 'PlayStation 4'),
    ('hades', 'PlayStation 5'),
    ('hades', 'Xbox One'),
    ('hades', 'Xbox Series X|S'),
    ('celeste', 'PC'),
    ('celeste', 'Nintendo Switch'),
    ('celeste', 'PlayStation 4'),
    ('celeste', 'Xbox One'),
    ('stardew-valley', 'PC'),
    ('stardew-valley', 'Nintendo Switch'),
    ('stardew-valley', 'PlayStation 4'),
    ('stardew-valley', 'Xbox One'),
    ('minecraft', 'PC'),
    ('minecraft', 'Nintendo Switch'),
    ('minecraft', 'PlayStation 4'),
    ('minecraft', 'Xbox One'),
    ('portal-2', 'PC'),
    ('half-life-2', 'PC'),
    ('dark-souls', 'PC'),
    ('super-mario-odyssey', 'Nintendo Switch'),
    ('red-dead-redemption-2', 'PC'),
    ('red-dead-redemption-2', 'PlayStation 4'),
    ('red-dead-redemption-2', 'Xbox One'),
    ('hollow-knight', 'PC'),
    ('hollow-knight', 'Nintendo Switch'),
    ('hollow-knight', 'PlayStation 4'),
    ('hollow-knight', 'Xbox One'),
    ('civilization-vi', 'PC'),
    ('civilization-vi', 'Nintendo Switch'),
    ('civilization-vi', 'PlayStation 4'),
    ('civilization-vi', 'Xbox One'),
    ('forza-horizon-5', 'PC'),
    ('forza-horizon-5', 'Xbox One'),
    ('forza-horizon-5', 'Xbox Series X|S'),
    ('forza-horizon-5', 'PlayStation 5'),
    ('rocket-league', 'PC'),
    ('rocket-league', 'Nintendo Switch'),
    ('rocket-league', 'PlayStation 4'),
    ('rocket-league', 'Xbox One'),
    ('resident-evil-4', 'PC'),
    ('resident-evil-4', 'PlayStation 4'),
    ('resident-evil-4', 'PlayStation 5'),
    ('resident-evil-4', 'Xbox Series X|S'),
    ('street-fighter-6', 'PC'),
    ('street-fighter-6', 'PlayStation 4'),
    ('street-fighter-6', 'PlayStation 5'),
    ('street-fighter-6', 'Xbox Series X|S'),
    ('baldurs-gate-3', 'PC'),
    ('baldurs-gate-3', 'PlayStation 5'),
    ('baldurs-gate-3', 'Xbox Series X|S'),
    ('disco-elysium', 'PC'),
    ('disco-elysium', 'PlayStation 4'),
    ('disco-elysium', 'PlayStation 5'),
    ('disco-elysium', 'Xbox One'),
    ('disco-elysium', 'Xbox Series X|S'),
    ('disco-elysium', 'Nintendo Switch')
) AS v (slug, platform)
JOIN games     g ON g.slug = v.slug
JOIN platforms p ON p.name = v.platform;

COMMIT;
