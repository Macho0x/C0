# GitHub syntax highlighting for Goop

GitHub uses [Linguist](https://github.com/github-linguist/linguist) for language detection and syntax highlighting. A custom `.gitattributes` entry **cannot** register a new language — Goop must be added to Linguist.

## Current status

| Layer | Status |
|---|---|
| TextMate grammar | [`syntaxes/goop.tmLanguage.json`](../../syntaxes/goop.tmLanguage.json) |
| Linguist samples | [`samples/`](samples/) (real project code, MIT) |
| Linguist metadata | [`goop.yml`](goop.yml) |
| GitHub web UI | **Pending** Linguist PR merge |

Until Goop is in Linguist, `.goop` files on github.com render as plain text.

## Submit to Linguist

From a clone of [github-linguist/linguist](https://github.com/github-linguist/linguist):

```bash
git clone https://github.com/github-linguist/linguist.git
cd linguist
script/bootstrap

# Add grammar from this repo (MIT/Apache-2.0)
script/add-grammar https://github.com/Macho0x/Goop

# Copy samples (do not use hello-world-only samples)
cp /path/to/Goop/docs/github-linguist/samples/*.goop samples/Goop/

# Add language entry — see goop.yml in this directory for the YAML block
# Edit lib/linguist/languages.yml (alphabetical, under Go)

script/update-ids
bundle exec rake samples
script/cibuild
```

Open a PR to `github-linguist/linguist` with:

- Link to in-the-wild `.goop` usage (this repo counts)
- Statement that samples are MIT-licensed
- Grammar license: MIT / Apache-2.0 (dual, same as Goop)

After merge, add to the Goop repo root:

```gitattributes
*.goop linguist-language=Goop linguist-detectable=true
```

## Interim workaround (optional)

If you need *some* highlighting before Linguist merges, map to F# (similar `match` / `let` / offside syntax):

```gitattributes
*.goop linguist-language=F# linguist-detectable=true
```

This is approximate — not Goop-accurate. Remove once `Goop` is in Linguist.
