-- public.web_spot definition
-- Drop table
-- DROP TABLE public.web_spot;
CREATE TABLE public.web_spot (
	id bigserial NOT NULL,
	"name" varchar(120) NOT NULL,
	created_at timestamptz NOT NULL,
	CONSTRAINT web_spot_pkey PRIMARY KEY (id)
);
-- public.web_reservation definition
-- Drop table
-- DROP TABLE public.web_reservation;
CREATE TABLE public.web_reservation (
	id bigserial NOT NULL,
	author varchar(200) NOT NULL,
	created_at timestamptz NOT NULL,
	start_at timestamptz NOT NULL,
	end_at timestamptz NOT NULL,
	spot_id int8 NOT NULL,
	guild_id varchar(255) NOT NULL,
	author_discord_id varchar(200) NOT NULL,
	CONSTRAINT unique_reservation_time_and_space_per_guild UNIQUE (start_at, end_at, spot_id, guild_id),
	CONSTRAINT web_reservation_pkey PRIMARY KEY (id),
	CONSTRAINT web_reservations_no_overlapping_ranges EXCLUDE USING gist (
		spot_id WITH =,
		guild_id WITH =,
		tstzrange(start_at, end_at) WITH &&
	),
	CONSTRAINT web_reservation_spot_id_6b297c19_fk_web_spot_id FOREIGN KEY (spot_id) REFERENCES public.web_spot(id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX web_reservation_spot_id_6b297c19 ON public.web_reservation USING btree (spot_id);
CREATE INDEX web_reservations_no_overlapping_ranges ON public.web_reservation USING gist (spot_id, guild_id, tstzrange(start_at, end_at));