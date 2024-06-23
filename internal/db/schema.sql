CREATE TABLE IF NOT EXISTS users (
    id INTEGER NOT NULL PRIMARY KEY,
    displayed_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stats (
    user_id INTEGER NOT NULL,
    chat_id INTEGER NOT NULL,
    svo_count INTEGER NOT NULL DEFAULT 0,
    zov_count INTEGER NOT NULL DEFAULT 0,
    likvidirovan_count INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, chat_id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS stats_chat_id_idx ON stats(chat_id);
