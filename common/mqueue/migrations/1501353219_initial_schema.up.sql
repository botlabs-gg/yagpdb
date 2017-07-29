BEGIN;

CREATE TABLE mqueue (
	id serial NOT NULL PRIMARY KEY,
	source text NOT NULL,
	source_id text NOT NULL,
	message_str text NOT NULL,
	message_embed text NOT NULL,
	channel text NOT NULL,
	processed boolean NOT NULL
);


COMMIT;
