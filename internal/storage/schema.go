package storage

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS documents (
    id              BLOB PRIMARY KEY,

    source_path     TEXT NOT NULL UNIQUE,
    filename        TEXT NOT NULL,
    extension       TEXT NOT NULL,

    size            INTEGER NOT NULL,
    modified_at     TEXT NOT NULL,

    fingerprint     TEXT NOT NULL,

    title           TEXT,
    document_type   TEXT NOT NULL,

    author          TEXT,
    creator         TEXT,
    intelligence_status TEXT NOT NULL DEFAULT 'ready',

    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entities (
    id                BLOB PRIMARY KEY,

    type              TEXT NOT NULL,
    value             TEXT NOT NULL,
    normalized_value  TEXT NOT NULL,

    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,

    UNIQUE(type, normalized_value)
);

CREATE TABLE IF NOT EXISTS document_entities (
    document_id  BLOB NOT NULL,
    entity_id    BLOB NOT NULL,
    role         TEXT NOT NULL DEFAULT '',

    PRIMARY KEY (document_id, entity_id, role),

    FOREIGN KEY (document_id)
        REFERENCES documents(id)
        ON DELETE CASCADE,

    FOREIGN KEY (entity_id)
        REFERENCES entities(id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS document_relationships (
    id                 BLOB PRIMARY KEY,

    document_id_a      BLOB NOT NULL,
    document_id_b      BLOB NOT NULL,

    relationship_type  TEXT NOT NULL,

    confidence         REAL,

    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL,

    UNIQUE (
        document_id_a,
        document_id_b,
        relationship_type
    ),

    FOREIGN KEY (document_id_a)
        REFERENCES documents(id)
        ON DELETE CASCADE,

    FOREIGN KEY (document_id_b)
        REFERENCES documents(id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS relationship_entities (
    relationship_id  BLOB NOT NULL,
    entity_id        BLOB NOT NULL,

    PRIMARY KEY (relationship_id, entity_id),

    FOREIGN KEY (relationship_id)
        REFERENCES document_relationships(id)
        ON DELETE CASCADE,

    FOREIGN KEY (entity_id)
        REFERENCES entities(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_documents_modified_at
    ON documents(modified_at);

CREATE INDEX IF NOT EXISTS idx_documents_document_type
    ON documents(document_type);

CREATE INDEX IF NOT EXISTS idx_entities_normalized_value
    ON entities(normalized_value);

CREATE INDEX IF NOT EXISTS idx_document_entities_entity
    ON document_entities(entity_id);

CREATE INDEX IF NOT EXISTS idx_document_relationships_a
    ON document_relationships(document_id_a);

CREATE INDEX IF NOT EXISTS idx_document_relationships_b
    ON document_relationships(document_id_b);

CREATE INDEX IF NOT EXISTS idx_relationship_entities_entity
    ON relationship_entities(entity_id);
`
