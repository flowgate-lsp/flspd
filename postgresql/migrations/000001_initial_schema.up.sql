-- Payments
CREATE TABLE public.payments (
    payment_hash bytea NOT NULL,
    payment_secret bytea NOT NULL,
    destination bytea NOT NULL,
    incoming_amount_msat bigint NOT NULL,
    outgoing_amount_msat bigint NOT NULL,
    funding_tx_id bytea NULL,
    funding_tx_outnum int NULL,
    tag jsonb NULL,
    opening_fee_params jsonb NULL,
    CONSTRAINT payments_pkey PRIMARY KEY (payment_hash)
);
CREATE INDEX probe_payment_hash ON public.payments (sha256('probing-01:' || payment_hash));
CREATE INDEX payments_funding_tx_idx ON public.payments USING btree (funding_tx_id, funding_tx_outnum);

-- Forwarding History
CREATE TABLE public.forwarding_history (
    identifier varchar NOT NULL,
    resolved_time bigint NOT NULL,
    nodeid bytea NOT NULL,
    chanid_in bigint NOT NULL,
    chanid_out bigint NOT NULL,
    amt_msat_in bigint NOT NULL,
    amt_msat_out bigint NOT NULL,
    UNIQUE(nodeid, identifier)
);
CREATE INDEX forwarding_history_nodeid_idx ON public.forwarding_history (nodeid);
CREATE INDEX forwarding_history_nodeid_resolved_time_idx ON public.forwarding_history USING btree (nodeid, resolved_time);
CREATE INDEX forwarding_history_resolved_time_idx ON public.forwarding_history USING btree (resolved_time);
CREATE INDEX forwarding_history_nodeid_chanid_in_idx ON public.forwarding_history USING btree (nodeid, chanid_in);
CREATE INDEX forwarding_history_nodeid_chanid_out_idx ON public.forwarding_history USING btree (nodeid, chanid_out);

-- Channels
CREATE TABLE public.channels (
    nodeid bytea NOT NULL,
    peerid bytea NOT NULL,
    alias_scid bigint NULL,
    confirmed_scid bigint NULL,
    funding_tx_id bytea NOT NULL,
    funding_tx_outnum bigint NOT NULL,
    first_seen timestamp NOT NULL,
    last_update timestamp NOT NULL,
    UNIQUE(nodeid, funding_tx_id, funding_tx_outnum)
);
CREATE INDEX channels_nodeid_idx ON public.channels USING btree (nodeid);
CREATE INDEX channels_peerid_idx ON public.channels USING btree (peerid);
CREATE INDEX channels_funding_tx_idx ON public.channels USING btree (funding_tx_id, funding_tx_outnum);
CREATE INDEX channels_nodeid_funding_tx_idx ON public.channels USING btree (nodeid, funding_tx_id, funding_tx_outnum);
CREATE INDEX channels_alias_scid_idx ON public.channels USING btree (alias_scid);
CREATE INDEX channels_confirmed_scid_idx ON public.channels USING btree (confirmed_scid);

-- New Channel Params
CREATE TABLE public.new_channel_params (
    validity int NOT NULL,
    params jsonb NOT NULL,
    token varchar
);
CREATE UNIQUE INDEX new_channel_params_token_validity_idx ON public.new_channel_params (token, validity);

INSERT INTO public.new_channel_params (validity, params)
 VALUES(259200, '{"min_msat": "12000000", "proportional": 7500, "max_idle_time": 4320, "max_client_to_self_delay": 432}'::jsonb);

INSERT INTO public.new_channel_params (validity, params)
 VALUES(3600, '{"min_msat": "10000000", "proportional": 7500, "max_idle_time": 4320, "max_client_to_self_delay": 432}'::jsonb);

-- Notification Subscriptions
CREATE TABLE public.notification_subscriptions (
    id bigserial primary key,
    pubkey bytea NOT NULL,
    url varchar NOT NULL,
    created_at bigint NOT NULL,
    refreshed_at bigint NOT NULL
);
CREATE INDEX notification_subscriptions_pubkey_idx ON public.notification_subscriptions (pubkey);
CREATE UNIQUE INDEX notification_subscriptions_pubkey_url_key ON public.notification_subscriptions (pubkey, url);

-- LSPS2 Schema
CREATE SCHEMA lsps2;
CREATE TABLE lsps2.buy_registrations (
    id uuid PRIMARY KEY, 
    lsp_id varchar NOT NULL,
    peer_id varchar NOT NULL,
    scid bigint NOT NULL,
    mode smallint NOT NULL,
    payment_size_msat bigint NULL,
    params_min_fee_msat bigint NOT NULL,
    params_proportional bigint NOT NULL,
    params_valid_until varchar NOT NULL,
    params_min_lifetime bigint NOT NULL,
    params_max_client_to_self_delay bigint NOT NULL,
    params_promise varchar NOT NULL,
    token VARCHAR NOT NULL
);
CREATE UNIQUE INDEX idx_lsps2_buy_registrations_scid ON lsps2.buy_registrations (scid);
CREATE INDEX idx_lsps2_buy_registrations_valid_until ON lsps2.buy_registrations (params_valid_until);

CREATE TABLE lsps2.bought_channels (
    id uuid PRIMARY KEY,
    registration_id uuid NOT NULL,
    funding_tx_id bytea NOT NULL,
    funding_tx_outnum bigint NOT NULL,
    fee_msat bigint NOT NULL,
    payment_size_msat bigint NOT NULL,
    is_completed boolean NOT NULL,
    CONSTRAINT fk_buy_registration
      FOREIGN KEY(registration_id)
      REFERENCES lsps2.buy_registrations(id)
      ON DELETE CASCADE
);
CREATE INDEX idx_lsps2_bought_channels_registration_id ON lsps2.bought_channels (registration_id);
CREATE INDEX lsps2_bought_channels_registration_id_funding_tx_idx ON lsps2.bought_channels USING btree (registration_id, funding_tx_id, funding_tx_outnum);

CREATE TABLE lsps2.promises (
    promise varchar PRIMARY KEY,
    token varchar NOT NULL,
    valid_until varchar NOT NULL
);
CREATE INDEX idx_lsps2_promises_valid_until ON lsps2.promises (valid_until);

-- Open Channel HTLCs
CREATE TABLE public.open_channel_htlcs (
    nodeid bytea NOT NULL,
    peerid bytea NOT NULL,
    funding_tx_id bytea NOT NULL,
    funding_tx_outnum bigint NOT NULL,
    forward_amt_msat bigint NOT NULL,
    original_amt_msat bigint NOT NULL,
    forward_time bigint NOT NULL,
    incoming_amt_msat bigint NOT NULL
);
