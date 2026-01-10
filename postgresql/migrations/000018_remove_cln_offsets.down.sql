CREATE TABLE public.cln_forwarding_history_offsets (
    nodeid bytea NOT NULL,
    last_created_index bigint NOT NULL,
    last_updated_index bigint NOT NULL,
    UNIQUE(nodeid)
);
