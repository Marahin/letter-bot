# Translating TibiaLoot.com

Every string the web panel shows lives in one of the JSON files in this directory,
one per language. To translate, you edit **one file**. You need no Go and no build
tools: edit the file, then open a merge request.

| file | language | state |
|---|---|---|
| `en.json` | English | source of truth, written by the maintainers |
| `pl.json` | Polski | written by the maintainers, pending native review |

The Discord bot's own replies stay in English (decision 33). These files cover the
web panel only.

## How to contribute

1. Pick your language's file. **Do not edit `en.json`** to translate: English is the
   source text, and a change there changes the app's own copy.
2. Change the values, never the keys. A key is a stable id the code references;
   renaming one silently drops the translation.
3. Open a merge request.

A key you leave out renders **English**, not a broken placeholder, so a partial
translation is always safe to merge.

## File format

```json
{
  "locale": "pl",
  "status": "machine-translated; pending native review",
  "reviewed_by": [],
  "messages": {
    "shell.nav.reservations": "Rezerwacje",
    "shell.switcher.server_count": {
      "one": "%d serwer",
      "few": "%d serwery",
      "many": "%d serwerów",
      "other": "%d serwera"
    }
  }
}
```

- **Keys** are `<area>.<component>.<thing>` (`shell.nav.reservations`,
  `landing.calculator.sub`, `error.not_found.title`), sorted alphabetically.
- **`%s` / `%d`** are placeholders the app fills in. Keep every placeholder the
  English text has. `%[1]s` / `%[2]s` pin an explicit order when you need to
  reorder two of them.
- **`{name}` placeholders** appear in the strings the browser fills in. Same rule:
  keep them, move them.
- **A literal percent sign is `%%`.** A lone `%` prints `%!(NOVERB)`. A test fails
  the build on a bare one.
- Do not add fields. An unknown field is a load error, so a typo cannot silently
  drop a section.

## Dates

Dates live in the `dates` block: 12 `months_short` (January first), 7
`weekdays_short` (Monday first), and optional `formats`. Use the form a month takes
**inside a date**. A format you leave out uses English's order with your names.
Placeholders: `{Wd}`, `{D}`, `{DD}`, `{Mon}`, `{YYYY}`, `{HH}`, `{mm}`. The calendar
widget's own month names are in `web/dist/l10n/<code>.js`.

## Plural forms

A message that counts something is an object of CLDR plural categories:

| language | categories | example |
|---|---|---|
| `en` | `one`, `other` | 1 server / 2 servers |
| `pl` | `one`, `few`, `many`, `other` | 1 serwer / 2 serwery / 5 serwerów |

`other` is **required**. In Polish, `21` is `many` (`21 serwerów`) and `22` is `few`
(`22 serwery`). The rules come from CLDR; you only fill in the forms.

## Glossary

These stay exactly as they are in every language:

| term | why |
|---|---|
| `TibiaLoot.com` | the site's name (the web panel) |
| `Letter` | the Discord bot's name |
| `Tibia` | game name |
| `TibiaData` | third-party service name |
| `Party Hunt Analyser` | Tibia's in-game window name; in Polish do not inflect it, let a noun carry the case (`z okna Party Hunt Analyser`) |
| `Discord` | third-party product name; inflect it in Polish (`na Discordzie`, `do Discorda`) |

Settled Polish wording, used everywhere in `pl.json`:

| English | Polish |
|---|---|
| respawn | resp (`resp`, `respy`, `respów`, `na respie`) |
| reservation | rezerwacja |
| Loot Calculator | kalkulator lootu |
| hunt | hunt |
| rank (a Discord role) | ranga |
| vocation | profesja |

Polish addresses the reader informally ("Ty"), and the guild in the plural ("Wasz
serwer").

## Reviewing a translation

While `reviewed_by` is empty, the language picker shows a quiet "Help us improve
this translation" line. A native speaker who read the whole file sets `"status"` to
`"native-reviewed"` and adds their handle to `"reviewed_by"`, in the same merge
request.

## Language selection

The app picks a language from `?lang=`, then the `letter_lang` cookie (set by the
language picker), then the browser's `Accept-Language`, then English. It never looks
at the visitor's IP.

## Where the keys are used

Every call site is a literal `i18n.T(ctx, "the.key")` or `i18n.N(ctx, "the.key", n)`.
A test fails when a key is referenced but missing from `en.json`, or present in
`en.json` but referenced nowhere.
