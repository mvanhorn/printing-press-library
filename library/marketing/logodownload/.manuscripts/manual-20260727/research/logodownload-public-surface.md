# LogoDownload Public Surface Research

Run ID: `manual-20260727`

## Target

`https://logodownload.org`

## Auth

No authentication is required for the supported flows.

## Public Search

The public website exposes WordPress search through:

```text
https://logodownload.org/?s=<query>
```

Observed result cards use:

- `article.grid-post`
- `h2.entry-title a` for the logo page title and URL
- `.post-thumbnail img` for the preview image URL

The CLI keeps extra selectors for older/common WordPress theme variants:

- `h2.title a`
- `h2.post-title a`
- `a[rel='bookmark']`
- `img.wp-post-image`

## Fallback API

The public WordPress search API is available at:

```text
https://logodownload.org/wp-json/wp/v2/search?search=<query>&subtype=post&per_page=<limit>
```

This endpoint returns post IDs, titles, and URLs. To recover a preview image when HTML parsing does not produce one, the CLI fetches:

```text
https://logodownload.org/wp-json/wp/v2/posts/<id>?_embed=wp:featuredmedia
```

Then it reads `_embedded["wp:featuredmedia"][0]`.

## Supported Operations

- Search by brand/company term.
- Preview returned `image_url` values in the terminal.
- Download selected returned `image_url` values to local storage.

## Safety Boundary

- No login.
- No bypassing protected content.
- No remote writes.
- Download is explicit and local only.
