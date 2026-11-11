-- +goose Up

-- Each week gets an editable resources page (sanitized HTML) in place of a
-- single link.
ALTER TABLE app.season_weeks
  ADD COLUMN resources_html text NOT NULL DEFAULT ''
    CONSTRAINT season_weeks_resources_html_length CHECK (char_length(resources_html) <= 50000);

UPDATE app.season_weeks
SET resources_html = '<p><a href="'
  || replace(replace(replace(replace(resource_url, '&', '&amp;'), '"', '&quot;'), '<', '&lt;'), '>', '&gt;')
  || '">Week Resources</a></p>'
WHERE resource_url <> '';

ALTER TABLE app.season_weeks DROP COLUMN resource_url;

-- +goose Down
ALTER TABLE app.season_weeks ADD COLUMN resource_url text NOT NULL DEFAULT '';

-- The first link on the page becomes the single URL again.
UPDATE app.season_weeks
SET resource_url = replace(replace(substring(resources_html from 'href="([^"]*)"'), '&quot;', '"'), '&amp;', '&')
WHERE resources_html ~ 'href="[^"]*"';

ALTER TABLE app.season_weeks DROP COLUMN resources_html;
