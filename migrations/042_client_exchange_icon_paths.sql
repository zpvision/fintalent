UPDATE client_exchange_dictionary_items
SET icon = CASE
    WHEN LOWER(icon) LIKE 'static/%' THEN '/' || icon
    ELSE '/static/' || icon
END,
updated_at = NOW()
WHERE icon <> ''
  AND icon !~* '^(?:/|https?://|data:)'
  AND icon !~ '\.\.'
  AND icon ~* '^(?:static/)?[a-z0-9][a-z0-9._/-]*\.(svg|png|jpe?g|webp|gif)$';
