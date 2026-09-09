-- Atlas-style SQLite table recreation
PRAGMA foreign_keys = off;
CREATE TABLE `new_signals` (`id` text NOT NULL PRIMARY KEY, `source_name` text NOT NULL);
INSERT INTO `new_signals` (`id`, `source_name`) SELECT `id`, `source_name` FROM `signals`;
DROP TABLE `signals`;
ALTER TABLE `new_signals` RENAME TO `signals`;
PRAGMA foreign_keys = on;
