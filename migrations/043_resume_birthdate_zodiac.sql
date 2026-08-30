ALTER TABLE resumes
    ADD COLUMN IF NOT EXISTS birth_day SMALLINT,
    ADD COLUMN IF NOT EXISTS birth_month SMALLINT,
    ADD COLUMN IF NOT EXISTS birth_year SMALLINT;

CREATE TABLE IF NOT EXISTS zodiac_signs (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    start_month SMALLINT NOT NULL CHECK (start_month BETWEEN 1 AND 12),
    start_day SMALLINT NOT NULL CHECK (start_day BETWEEN 1 AND 31),
    end_month SMALLINT NOT NULL CHECK (end_month BETWEEN 1 AND 12),
    end_day SMALLINT NOT NULL CHECK (end_day BETWEEN 1 AND 31),
    icon TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO zodiac_signs(code,name,start_month,start_day,end_month,end_day,icon,sort_order) VALUES
('aries','Овен',3,21,4,19,'/static/icons/zodiac/aries.svg',0),
('taurus','Телец',4,20,5,20,'/static/icons/zodiac/taurus.svg',1),
('gemini','Близнецы',5,21,6,20,'/static/icons/zodiac/gemini.svg',2),
('cancer','Рак',6,21,7,22,'/static/icons/zodiac/cancer.svg',3),
('leo','Лев',7,23,8,22,'/static/icons/zodiac/leo.svg',4),
('virgo','Дева',8,23,9,22,'/static/icons/zodiac/virgo.svg',5),
('libra','Весы',9,23,10,22,'/static/icons/zodiac/libra.svg',6),
('scorpio','Скорпион',10,23,11,21,'/static/icons/zodiac/scorpio.svg',7),
('sagittarius','Стрелец',11,22,12,21,'/static/icons/zodiac/sagittarius.svg',8),
('capricorn','Козерог',12,22,1,19,'/static/icons/zodiac/capricorn.svg',9),
('aquarius','Водолей',1,20,2,18,'/static/icons/zodiac/aquarius.svg',10),
('pisces','Рыбы',2,19,3,20,'/static/icons/zodiac/pisces.svg',11)
ON CONFLICT(code) DO NOTHING;
