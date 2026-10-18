# TibiaLoot.com and Letter vocabulary

The words the web panel (TibiaLoot.com), the bot (Letter) and the code use for
the same things. The
Polish column is the settled wording of `pl.json` (see
`internal/infrastructure/i18n/locales/README.md`).

| Term (UI, English) | Polish | In code | Meaning |
|---|---|---|---|
| Respawn | Resp (`respy`, `respów`) | `spot`, table `web_spot` | A hunting place that members book. It belongs to one server. The bot's `/book` option is `respawn`. |
| Archived respawn | Zarchiwizowany resp | `spot.ArchivedAt` | A respawn removed from the list that keeps its reservations and stats. It can be restored. |
| Reservation | Rezerwacja | `reservation`, table `web_reservation` | One booking of a respawn from a start to an end time, at most 3 hours. |
| Author | Autor | `author`, `author_discord_id` | Who booked. The text holds the Tibia character names, separated by `/`. The Discord id is empty for a free-text author. |
| Free-text author | Autor wpisany ręcznie | `author_discord_id = ''` | An author that a manager typed, with no Discord account. No quota, no owner. |
| Overbook | Nadpisz | `Overbook`, `ClippedOrRemovedReservation` | Book over existing reservations. They are shortened or removed, and their authors get a Discord message. |
| Summary | Podsumowanie | `SummaryService`, `letter-summary` | The bot's message with the upcoming reservations and a chart, in the summary channel. |
| Region | Region | `regions`, `web_spot.region` | A group of respawns for the summary, e.g. "Port Hope & Tiquanda". A respawn without one is in "Other" (Inne). |
| Region summaries | Podsumowania regionów | `guilds.region_summaries`, `summarylayout` | The feature that posts one region channel per region and an index in the summary channel. In use only when the setting is on, Discord does not block the channels, and an active respawn has a region. |
| Region channel | Kanał regionu | `summary_region_channels`, `letter-<key>` | The bot's channel with the summary of one region, in the category "Letter respawns". |
| Respawn image | Obrazek respa | `web_spot.image_url` | An optional picture of a respawn, shown in its region channel. |
| Booking announcement | Ogłoszenie rezerwacji (bot only, in English) | `letter_booked`, `BookingMade` | The bot's public message in the command channel about a new booking. |
| Queue | Kolejka (bot only, in English) | `spot_queue_entries`, `/queue`, `SpotQueueService` | A member who waits for a taken respawn, with the length they want to hunt. |
| Queue offer | Propozycja z kolejki (bot only, in English) | `offered_at`, `DueOffers` | The bot's DM that a queued respawn is free, with Book and Cancel for 10 minutes. |
| Booking wizard | Kreator rezerwacji (bot only, in English) | `reservationforms`, `RespawnPicker`, `TimePicker` | The bot's private message with lists where a member picks the respawn, the start and the length to book or edit a reservation. |
| My reservations | Moje rezerwacje (bot only, in English) | `actionMine`, `/reservations` | The bot's private list of the member's upcoming reservations, with Edit and Cancel buttons. |
| Command channel | Kanał komend | `command_channel_id` | The only channel for `/book` and `/unbook` when set. |
| Summary channel | Kanał podsumowania | `summary_channel_id` | Where the bot posts the summary. Empty = `#letter-summary`. |
| Premium | Premium | `guilds.premium`, `premium_forever` | A server where the bot works. Without premium the bot is inactive. |
| Premium forever | Premium na zawsze | `premium_forever` | Premium that the web cannot turn off (Celesta Community). |
| Rank | Ranga | role id lists on `guilds` | A Discord role chosen in Settings for one of the four rights below. |
| Manage rank | Ranga zarządzająca | `manage_role_ids`, `Capabilities.Manage` | Manages respawns and every reservation. Owners and administrators always manage. |
| View rank | Ranga tylko do podglądu | `view_role_ids`, `Capabilities.View` | Sees the reservations and the stats. Books only with the reserve right. |
| Reserve rank | Ranga rezerwująca | `reserve_role_ids`, `Capabilities.Reserve` | Can book. No reserve rank set = every member can book. |
| Overbook rank | Ranga nadpisująca rezerwacje | `overbook_role_ids`, `Capabilities.Overbook` | Can overbook. No overbook rank set = the `Postman` role can. |
| Admin (of a server) | Administrator | `Capabilities.Admin` | The server owner or a member with the Administrator permission. Opens Settings and Channels. |
| Site admin | Administrator serwisu | `WEB_ADMIN_DISCORD_IDS` | A TibiaLoot.com operator: full access to every server, turns premium on and off. |
| Player | Gracz | `author_discord_id` | A Discord user who books. Stats group their reservations. |
| Character | Postać | `experience.CharacterKey` | One Tibia character from an author text, compared as `lower(trim(name))`. |
| World | Świat | `guilds_world.world_name` | The Tibia world of a server. The experience job reads its highscores. |
| Experience gain | Zdobyte doświadczenie | `reservation_experience.gain` | The experience a character gained during a reservation, from highscore snapshots. Can be negative after a death. |
| No data | Brak danych | `status = 'no_data'` | No experience figure: the character was outside the world top 1000 or not tracked. Never shown as 0. |
| Booked hours | Zarezerwowane godziny | `stats.Totals.Seconds` | The length of the reservations, not measured play time. |
| Loot Calculator | Kalkulator lootu | `lootcalc` | Splits a Tibia party hunt session into bank transfers. |
| Loot session | Sesja polowania (`sesja`) | `loot_sessions`, `lootsession` | A Party Hunt session counted from the Loot Calculator: its times and each member's figures, not the pasted text. It belongs to at most one reservation (`reservation_loot`). |
| Profit | Zysk | `stats.Totals.Profit` | The balance of the counted sessions. On a character row, the character's equal share. Can be negative (a loss). |
| Profit per hour | Zysk na godzinę (`Zysk/h`) | `Totals.ProfitPerHour` | Profit divided by the session time, not the booked time. |
| World page | Strona świata | `/worlds/{world}`, `worldstats` | The public stats of one Tibia world, across the servers that set it. |
| Spawn | Resp (`Respy` in the menu) | `/spawns/{key}`, `stats.SpawnRow` | A respawn name across servers, keyed by `lower(trim(name))`. A server's own list keeps "Respawns". |
| Party Hunt Analyser | Party Hunt Analyser (not translated; `z okna Party Hunt Analyser`) | `lootcalc.Parse` input | Tibia's in-game window whose copied text the Loot Calculator reads. |
| Public stats | Statystyki publiczne | `web.PublicView`, `pageView.Public` | The Stats pages as anyone sees them without being a member. |
| Premium lock | Blokada Premium | `web.PremiumRequired` | The page every server page shows on a server without premium. |
| Community Discord | Discord społeczności | `DISCORD_INVITE_LINK`, `web.SupportLink` | The TibiaLoot.com Discord server where users get help and premium. |
