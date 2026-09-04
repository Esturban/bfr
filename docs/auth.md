# Auth

`bfr` reads `BUFFER_API_KEY` from the environment, or from a `.env` file at
the repo root (gitignored, never committed). Generate a token from your
Buffer account's API access / developer app settings. It is never passed as
a flag, and never echoed or logged.

```sh
# .env
BUFFER_API_KEY=your-token-here
```

## Image verbs need more

`image` and `draft-image` upload a local file to Google Drive before
attaching it, so they additionally require:

- `BUFFER_DRIVE_ACCOUNT`: a [`gog`](https://github.com/Esturban)-authenticated
  Google Drive account email, used to upload the image and share it before
  attaching it to the post. No default, it must be set explicitly.
- The `gog` and `sips` CLIs on `PATH` (`sips` is macOS-native, used to
  convert the source image to real JPEG bytes before upload).

`attach-image` does **not** need any of this: it takes an already-public
URL, not a local file, so it does no Drive upload of its own.

## Optional

`BUFFER_CACHE_FILE` overrides where the channel cache is written (default:
`.bfr-channels.json` next to the repo root). Written by `bfr channels`,
read by every command that takes a channel name instead of a raw id.
