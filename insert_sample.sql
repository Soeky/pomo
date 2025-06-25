-- SQL script to insert 5 sample sessions for today into the sessions table

BEGIN TRANSACTION;

INSERT INTO sessions (type, topic, start_time, end_time, duration) VALUES
  ('focus',    'Study Go',      datetime('now', '-5 hours'),          datetime('now', '-4 hours', '-30 minutes'), 1800),
  ('break',    '',              datetime('now', '-4 hours', '-30 minutes'), datetime('now', '-4 hours'),             1800),
  ('focus',    'Write Tests',   datetime('now', '-3 hours'),           datetime('now', '-2 hours', '-45 minutes'), 2100),
  ('break',    '',              datetime('now', '-2 hours', '-45 minutes'), datetime('now', '-2 hours'),             2700),
  ('focus',    'Code Review',   datetime('now', '-1 hours', '-30 minutes'), datetime('now', '-1 hours'),            1800);

COMMIT;
