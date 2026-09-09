-- Atlas dry-run: compatible expand
ALTER TABLE `signals` ADD COLUMN `language` text NULL;
CREATE TABLE `source_health` (`source` text NOT NULL, `observed_at` text NOT NULL, PRIMARY KEY (`source`));
CREATE UNIQUE INDEX `source_health_source` ON `source_health` (`source`);
CREATE INDEX `idx_source_health_observed` ON `source_health` (`observed_at`);
