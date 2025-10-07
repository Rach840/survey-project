-- Удобные типы (можно убрать, если не нужны)
CREATE EXTENSION IF NOT EXISTS citext;
-- 1) Аккаунты (владельцы/преподаватели)
CREATE TABLE accounts (
                          id          BIGSERIAL PRIMARY KEY,
                          email       CITEXT UNIQUE NOT NULL,
                          passHash    TEXT NOT NULL ,
                          full_name   TEXT NOT NULL,
                          role        TEXT NOT NULL CHECK (role IN ('ADMIN','QUESTIONER')),
                          created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
                          disabled_at TIMESTAMPTZ
);

-- 2) Шаблоны: черновик и опубликованная версия в JSONB
CREATE TABLE form_templates (
                                id                   BIGSERIAL PRIMARY KEY,
                                owner_id             BIGINT NOT NULL REFERENCES accounts(id),
                                title                TEXT NOT NULL,
                                description          TEXT,
                                version              INT NOT NULL DEFAULT 1,
                                status               TEXT NOT NULL CHECK (status IN ('draft','published')),
                                draft_schema_json    JSONB NOT NULL DEFAULT '{}'::jsonb, -- редактируемый черновик
                                published_schema_json JSONB,                              -- фиксируется при публикации
                                updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
                                published_at         TIMESTAMPTZ
);

-- 3) Проведение: хранит СНАПШОТ формы (чтобы дальше не зависеть от редактирования)
CREATE TABLE surveys (
                         id                 BIGSERIAL PRIMARY KEY,
                         owner_id           BIGINT NOT NULL REFERENCES accounts(id),
                         template_id        BIGINT REFERENCES form_templates(id),
                         snapshot_version   INT NOT NULL,                   -- с какой версии шаблона взяли снапшот
                         form_snapshot_json JSONB NOT NULL,                 -- целиком структура формы
                         title              TEXT NOT NULL,
                         mode               TEXT NOT NULL CHECK (mode IN ('admin','bot')),
                         status             TEXT NOT NULL CHECK (status IN ('draft','open','closed','archived')),
                         max_participants   INT CHECK (max_participants IS NULL OR max_participants > 0),
                         public_slug        TEXT UNIQUE,                    -- ссылка регистрации (бот-режим)
                         starts_at          TIMESTAMPTZ,
                         ends_at            TIMESTAMPTZ,
                         created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4) ЕДИНАЯ таблица для инвайтов/регистраций/участников/доступов
CREATE TABLE enrollments (
                             id                BIGSERIAL PRIMARY KEY,
                             survey_id         BIGINT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
                             source            TEXT  NOT NULL CHECK (source IN ('admin','bot')), -- откуда пришёл
                             full_name         TEXT  NOT NULL,
                             email             CITEXT,
                             phone             TEXT,
                             telegram_chat_id  BIGINT,  -- если привязан Telegram
                             state             TEXT  NOT NULL CHECK (
                                 state IN ('invited','pending','approved','active','removed','rejected','expired')
                                 ),
                             invited_by        BIGINT REFERENCES accounts(id),
                             approved_by       BIGINT REFERENCES accounts(id),
                             approved_at       TIMESTAMPTZ,
    -- единый токен: и для инвайта, и для deep-link бота, и для доступа к форме
                             token_hash        BYTEA UNIQUE,        -- хранить ХЭШ (sha256), не сам токен!
                             token_expires_at  TIMESTAMPTZ,
                             use_limit         INT NOT NULL DEFAULT 1,
                             used_count        INT NOT NULL DEFAULT 0,
                             created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Полезные уникальные индексы (необязательно, но помогают)
CREATE UNIQUE INDEX uniq_enroll_telegram
    ON enrollments(survey_id, telegram_chat_id)
    WHERE telegram_chat_id IS NOT NULL AND state <> 'removed';

CREATE UNIQUE INDEX uniq_enroll_email
    ON enrollments(survey_id, email)
    WHERE email IS NOT NULL AND state <> 'removed';

-- 5) Попытка прохождения (одна на участника по умолчанию)
CREATE TABLE responses (
                           id              BIGSERIAL PRIMARY KEY,
                           survey_id       BIGINT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
                           enrollment_id   BIGINT NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
                           state           TEXT NOT NULL CHECK (state IN ('in_progress','submitted')),
                           channel         TEXT CHECK (channel IN ('web','tg_webapp','api')),
                           started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
                           submitted_at    TIMESTAMPTZ,
                           UNIQUE (survey_id, enrollment_id) -- по желанию: если нужны повторные попытки, уберите
);

-- 6) Ответы: без нормализации вопросов/опций; идентификация по code из снапшота
CREATE TABLE answers (
                         id             BIGSERIAL PRIMARY KEY,
                         response_id    BIGINT NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
                         question_code  TEXT   NOT NULL,   -- код вопроса из form_snapshot_json
                         section_code   TEXT,              -- если нужно различать группы
                         repeat_path    TEXT   NOT NULL DEFAULT '', -- для повторяемых секций (например "parents:0")
                       value_text     TEXT,
                         value_number   NUMERIC(18,6),
                         value_bool     BOOLEAN,
                         value_date     DATE,
                         value_datetime TIMESTAMPTZ,
                         value_json     JSONB,             -- тут: {options:[]}, {matrix:{…}}, {files:[…]} и т.п.
                         created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
                         UNIQUE (response_id, question_code, repeat_path)
);

CREATE INDEX idx_answers_by_question_code ON answers(question_code);
CREATE INDEX idx_answers_value_json_gin ON answers USING GIN (value_json);


