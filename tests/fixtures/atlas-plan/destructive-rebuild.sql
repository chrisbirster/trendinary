PRAGMA foreign_keys = OFF;
CREATE TABLE `new_signals` (`id` text PRIMARY KEY, `source_name` text NOT NULL);
INSERT INTO `new_signals` SELECT `id`, `source_name` FROM `signals`;
DROP TABLE `signals`;
ALTER TABLE `new_signals` RENAME TO `signals`;
PRAGMA foreign_keys = ON;
