CREATE TABLE links (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    target_url TEXT NOT NULL,
    title TEXT CHECK (title IS NULL OR char_length(title) BETWEEN 1 AND 120),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE link_stats (
    link_id BIGINT PRIMARY KEY REFERENCES links(id) ON DELETE CASCADE,
    click_count BIGINT NOT NULL DEFAULT 0,
    last_clicked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
