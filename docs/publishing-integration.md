# Publishing integration

Trendinary keeps human admin authentication separate from machine-to-machine publishing access.

## Human admin

`/admin/*` and `/api/v1/admin/*` continue to use HTTP Basic Auth:

```text
username: admin
password: TRENDINARY_ADMIN_PASSWORD
```

Do not copy `TRENDINARY_ADMIN_PASSWORD` into another application.

## Machine publishing access

A publishing application such as `chrisbirster.com` may read only explicitly approved editorial notes through:

```text
GET /api/v1/integrations/notes?worth_sharing=true
Authorization: Bearer <TRENDINARY_INTEGRATION_TOKEN>
```

`TRENDINARY_INTEGRATION_TOKEN` is optional. When it is unset, the integration endpoint returns `503` and the rest of Trendinary continues to run normally.

The endpoint is intentionally read-only and always limits output to saved/consumed items whose note has `worth_sharing=true`. It cannot be used to read the inbox, quality labels, newsletter drafts, private unapproved notes, or mutate editorial state.

The response is a compact publishing contract:

```json
[
  {
    "id": "content-...",
    "title": "Example",
    "url": "https://example.com/story",
    "publisher": "Example",
    "contentType": "article",
    "note": "Why this is worth sharing"
  }
]
```

Responses send `Cache-Control: no-store`.

The human admin notes endpoint also honors its filter directly:

```text
GET /api/v1/admin/notes?worth_sharing=true
```

That endpoint still requires the human Basic Auth credential and is not the recommended application-to-application interface.
