# iCloud CLI Research

## API

iCloud Photos data is stored in a local SQLite database at:
`~/Pictures/Photos Library.photoslibrary/database/Photos.sqlite`

This database is a CoreData store with the following tables used by this CLI:
- `ZASSET` — one row per media item (photo or video)
- `ZADDITIONALASSETATTRIBUTES` — extended metadata including original file size and filename

Key fields:
- `ZASSET.ZUUID` — unique identifier, used as the delete handle
- `ZASSET.ZKIND` — 0=photo, 1=video
- `ZASSET.ZDATECREATED` — CoreData timestamp (seconds since 2001-01-01)
- `ZASSET.ZTRASHEDSTATE` — 0=active, 1=in Recently Deleted
- `ZADDITIONALASSETATTRIBUTES.ZORIGINALFILESIZE` — original file size in bytes
- `ZADDITIONALASSETATTRIBUTES.ZORIGINALFILENAME` — original filename

## Deletion

There is no public iCloud API for deletion. The only supported path that
preserves iCloud sync and the 30-day recovery window is AppleScript via
`osascript`, driving Photos.app's `delete media items` handler.

## Auth

No authentication required. The database is a local file owned by the
current macOS user. Read access requires the Terminal (or running app) to
have Full Disk Access in System Settings → Privacy & Security.
