-- Numbers 078–085 are reserved for Academy on its separate development branch.
ALTER TABLE courses ADD COLUMN IF NOT EXISTS learning_outcomes JSONB NOT NULL DEFAULT '[]'::jsonb
 CHECK (jsonb_typeof(learning_outcomes) = 'array');
