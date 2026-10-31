-- CUSTOM: QxAPI client update feed and uploaded artifacts.
CREATE TABLE IF NOT EXISTS client_releases (
    id BIGSERIAL PRIMARY KEY,
    product VARCHAR(32) NOT NULL DEFAULT 'qxapi',
    version VARCHAR(64) NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    manifest JSONB NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('draft', 'published', 'withdrawn')),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product, version)
);
CREATE TABLE IF NOT EXISTS client_release_files (
    id BIGSERIAL PRIMARY KEY,
    release_id BIGINT NOT NULL REFERENCES client_releases(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    size BIGINT NOT NULL CHECK (size > 0),
    sha256 CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (release_id, name)
);
