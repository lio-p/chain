package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Type migration describes a single migration.
type migration struct {
	Name      string
	SQL       string
	Hash      string    // set in init
	AppliedAt time.Time // set in loadStatus
}

func init() {
	for i, m := range migrations {
		h := sha256.Sum256([]byte(m.SQL))
		migrations[i].Hash = hex.EncodeToString(h[:])
	}
}

var migrations = []migration{
	{Name: "2017-02-03.0.core.schema-snapshot.sql", SQL: "--\n-- PostgreSQL database dump\n--\n\n-- Dumped from database version 9.5.5\n-- Dumped by pg_dump version 9.5.5\n\nSET statement_timeout = 0;\nSET lock_timeout = 0;\nSET client_encoding = 'UTF8';\nSET standard_conforming_strings = on;\nSET check_function_bodies = false;\nSET client_min_messages = warning;\nSET row_security = off;\n\n--\n-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: -\n--\n\nCREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;\n\n\n--\n--\n\n\n\nSET search_path = public, pg_catalog;\n\n--\n-- Name: access_token_type; Type: TYPE; Schema: public; Owner: -\n--\n\nCREATE TYPE access_token_type AS ENUM (\n    'client',\n    'network'\n);\n\n\n--\n-- Name: b32enc_crockford(bytea); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION b32enc_crockford(src bytea) RETURNS text\n    LANGUAGE plpgsql IMMUTABLE\n    AS $$\n\t-- Adapted from the Go package encoding/base32.\n\t-- See https://golang.org/src/encoding/base32/base32.go.\n\t-- NOTE(kr): this function does not pad its output\nDECLARE\n\t-- alphabet is the base32 alphabet defined\n\t-- by Douglas Crockford. It preserves lexical\n\t-- order and avoids visually-similar symbols.\n\t-- See http://www.crockford.com/wrmg/base32.html.\n\talphabet text := '0123456789ABCDEFGHJKMNPQRSTVWXYZ';\n\tdst text := '';\n\tn integer;\n\tb0 integer;\n\tb1 integer;\n\tb2 integer;\n\tb3 integer;\n\tb4 integer;\n\tb5 integer;\n\tb6 integer;\n\tb7 integer;\nBEGIN\n\tFOR r IN 0..(length(src)-1) BY 5\n\tLOOP\n\t\tb0:=0; b1:=0; b2:=0; b3:=0; b4:=0; b5:=0; b6:=0; b7:=0;\n\n\t\t-- Unpack 8x 5-bit source blocks into an 8 byte\n\t\t-- destination quantum\n\t\tn := length(src) - r;\n\t\tIF n >= 5 THEN\n\t\t\tb7 := get_byte(src, r+4) & 31;\n\t\t\tb6 := get_byte(src, r+4) >> 5;\n\t\tEND IF;\n\t\tIF n >= 4 THEN\n\t\t\tb6 := b6 | (get_byte(src, r+3) << 3) & 31;\n\t\t\tb5 := (get_byte(src, r+3) >> 2) & 31;\n\t\t\tb4 := get_byte(src, r+3) >> 7;\n\t\tEND IF;\n\t\tIF n >= 3 THEN\n\t\t\tb4 := b4 | (get_byte(src, r+2) << 1) & 31;\n\t\t\tb3 := (get_byte(src, r+2) >> 4) & 31;\n\t\tEND IF;\n\t\tIF n >= 2 THEN\n\t\t\tb3 := b3 | (get_byte(src, r+1) << 4) & 31;\n\t\t\tb2 := (get_byte(src, r+1) >> 1) & 31;\n\t\t\tb1 := (get_byte(src, r+1) >> 6) & 31;\n\t\tEND IF;\n\t\tb1 := b1 | (get_byte(src, r) << 2) & 31;\n\t\tb0 := get_byte(src, r) >> 3;\n\n\t\t-- Encode 5-bit blocks using the base32 alphabet\n\t\tdst := dst || substr(alphabet, b0+1, 1);\n\t\tdst := dst || substr(alphabet, b1+1, 1);\n\t\tIF n >= 2 THEN\n\t\t\tdst := dst || substr(alphabet, b2+1, 1);\n\t\t\tdst := dst || substr(alphabet, b3+1, 1);\n\t\tEND IF;\n\t\tIF n >= 3 THEN\n\t\t\tdst := dst || substr(alphabet, b4+1, 1);\n\t\tEND IF;\n\t\tIF n >= 4 THEN\n\t\t\tdst := dst || substr(alphabet, b5+1, 1);\n\t\t\tdst := dst || substr(alphabet, b6+1, 1);\n\t\tEND IF;\n\t\tIF n >= 5 THEN\n\t\t\tdst := dst || substr(alphabet, b7+1, 1);\n\t\tEND IF;\n\tEND LOOP;\n\tRETURN dst;\nEND;\n$$;\n\n\n--\n-- Name: next_chain_id(text); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION next_chain_id(prefix text) RETURNS text\n    LANGUAGE plpgsql\n    AS $$\n\t-- Adapted from the technique published by Instagram.\n\t-- See http://instagram-engineering.tumblr.com/post/10853187575/sharding-ids-at-instagram.\nDECLARE\n\tour_epoch_ms bigint := 1433333333333; -- do not change\n\tseq_id bigint;\n\tnow_ms bigint;     -- from unix epoch, not ours\n\tshard_id int := 4; -- must be different on each shard\n\tn bigint;\nBEGIN\n\tSELECT nextval('chain_id_seq') % 1024 INTO seq_id;\n\tSELECT FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000) INTO now_ms;\n\tn := (now_ms - our_epoch_ms) << 23;\n\tn := n | (shard_id << 10);\n\tn := n | (seq_id);\n\tRETURN prefix || b32enc_crockford(int8send(n));\nEND;\n$$;\n\n\nSET default_tablespace = '';\n\nSET default_with_oids = false;\n\n--\n-- Name: access_tokens; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE access_tokens (\n    id text NOT NULL,\n    sort_id text DEFAULT next_chain_id('at'::text),\n    type access_token_type NOT NULL,\n    hashed_secret bytea NOT NULL,\n    created timestamp with time zone DEFAULT now() NOT NULL\n);\n\n\n--\n-- Name: account_control_program_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE account_control_program_seq\n    START WITH 10001\n    INCREMENT BY 10000\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: account_control_programs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE account_control_programs (\n    signer_id text NOT NULL,\n    key_index bigint NOT NULL,\n    control_program bytea NOT NULL,\n    change boolean NOT NULL,\n    expires_at timestamp with time zone\n);\n\n\n--\n-- Name: account_utxos; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE account_utxos (\n    tx_hash bytea NOT NULL,\n    index integer NOT NULL,\n    asset_id bytea NOT NULL,\n    amount bigint NOT NULL,\n    account_id text NOT NULL,\n    control_program_index bigint NOT NULL,\n    control_program bytea NOT NULL,\n    confirmed_in bigint NOT NULL,\n    output_id bytea NOT NULL\n);\n\n\n--\n-- Name: accounts; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE accounts (\n    account_id text NOT NULL,\n    tags jsonb,\n    alias text\n);\n\n\n--\n-- Name: annotated_accounts; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_accounts (\n    id text NOT NULL,\n    alias text,\n    keys jsonb NOT NULL,\n    quorum integer NOT NULL,\n    tags jsonb NOT NULL\n);\n\n\n--\n-- Name: annotated_assets; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_assets (\n    id bytea NOT NULL,\n    sort_id text NOT NULL,\n    alias text,\n    issuance_program bytea NOT NULL,\n    keys jsonb NOT NULL,\n    quorum integer NOT NULL,\n    definition jsonb NOT NULL,\n    tags jsonb NOT NULL,\n    local boolean NOT NULL\n);\n\n\n--\n-- Name: annotated_inputs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_inputs (\n    tx_hash bytea NOT NULL,\n    index integer NOT NULL,\n    type text NOT NULL,\n    asset_id bytea NOT NULL,\n    asset_alias text NOT NULL,\n    asset_definition jsonb NOT NULL,\n    asset_tags jsonb NOT NULL,\n    asset_local boolean NOT NULL,\n    amount bigint NOT NULL,\n    account_id text,\n    account_alias text,\n    account_tags jsonb,\n    issuance_program bytea NOT NULL,\n    reference_data jsonb NOT NULL,\n    local boolean NOT NULL\n);\n\n\n--\n-- Name: annotated_outputs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_outputs (\n    block_height bigint NOT NULL,\n    tx_pos integer NOT NULL,\n    output_index integer NOT NULL,\n    tx_hash bytea NOT NULL,\n    timespan int8range NOT NULL,\n    output_id bytea NOT NULL,\n    type text NOT NULL,\n    purpose text NOT NULL,\n    asset_id bytea NOT NULL,\n    asset_alias text NOT NULL,\n    asset_definition jsonb NOT NULL,\n    asset_tags jsonb NOT NULL,\n    asset_local boolean NOT NULL,\n    amount bigint NOT NULL,\n    account_id text,\n    account_alias text,\n    account_tags jsonb,\n    control_program bytea NOT NULL,\n    reference_data jsonb NOT NULL,\n    local boolean NOT NULL\n);\n\n\n--\n-- Name: annotated_txs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_txs (\n    block_height bigint NOT NULL,\n    tx_pos integer NOT NULL,\n    tx_hash bytea NOT NULL,\n    data jsonb NOT NULL,\n    \"timestamp\" timestamp with time zone NOT NULL,\n    block_id bytea NOT NULL,\n    local boolean NOT NULL,\n    reference_data jsonb NOT NULL\n);\n\n\n--\n-- Name: asset_tags; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE asset_tags (\n    asset_id bytea NOT NULL,\n    tags jsonb\n);\n\n\n--\n-- Name: assets; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE assets (\n    id bytea NOT NULL,\n    created_at timestamp with time zone DEFAULT now() NOT NULL,\n    sort_id text DEFAULT next_chain_id('asset'::text) NOT NULL,\n    issuance_program bytea NOT NULL,\n    client_token text,\n    initial_block_hash bytea NOT NULL,\n    signer_id text,\n    definition bytea NOT NULL,\n    alias text,\n    first_block_height bigint,\n    vm_version bigint NOT NULL\n);\n\n\n--\n-- Name: assets_key_index_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE assets_key_index_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: block_processors; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE block_processors (\n    name text NOT NULL,\n    height bigint DEFAULT 0 NOT NULL\n);\n\n\n--\n-- Name: blocks; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE blocks (\n    block_hash bytea NOT NULL,\n    height bigint NOT NULL,\n    data bytea NOT NULL,\n    header bytea NOT NULL\n);\n\n\n--\n-- Name: chain_id_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE chain_id_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: config; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE config (\n    singleton boolean DEFAULT true NOT NULL,\n    is_signer boolean,\n    is_generator boolean,\n    blockchain_id bytea NOT NULL,\n    configured_at timestamp with time zone NOT NULL,\n    generator_url text DEFAULT ''::text NOT NULL,\n    block_pub text DEFAULT ''::text NOT NULL,\n    remote_block_signers bytea DEFAULT '\\x'::bytea NOT NULL,\n    generator_access_token text DEFAULT ''::text NOT NULL,\n    max_issuance_window_ms bigint,\n    id text NOT NULL,\n    block_hsm_url text DEFAULT ''::text,\n    block_hsm_access_token text DEFAULT ''::text,\n    CONSTRAINT config_singleton CHECK (singleton)\n);\n\n\n--\n-- Name: generator_pending_block; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE generator_pending_block (\n    singleton boolean DEFAULT true NOT NULL,\n    data bytea NOT NULL,\n    CONSTRAINT generator_pending_block_singleton CHECK (singleton)\n);\n\n\n--\n-- Name: leader; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE leader (\n    singleton boolean DEFAULT true NOT NULL,\n    leader_key text NOT NULL,\n    expiry timestamp with time zone DEFAULT '1970-01-01 00:00:00-08'::timestamp with time zone NOT NULL,\n    address text NOT NULL,\n    CONSTRAINT leader_singleton CHECK (singleton)\n);\n\n--\n-- Name: mockhsm_sort_id_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE mockhsm_sort_id_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: mockhsm; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE mockhsm (\n    pub bytea NOT NULL,\n    prv bytea NOT NULL,\n    alias text,\n    sort_id bigint DEFAULT nextval('mockhsm_sort_id_seq'::regclass) NOT NULL,\n    key_type text DEFAULT 'chain_kd'::text NOT NULL\n);\n\n\n--\n-- Name: query_blocks; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE query_blocks (\n    height bigint NOT NULL,\n    \"timestamp\" bigint NOT NULL\n);\n\n\n--\n-- Name: signed_blocks; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE signed_blocks (\n    block_height bigint NOT NULL,\n    block_hash bytea NOT NULL\n);\n\n\n--\n-- Name: signers; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE signers (\n    id text NOT NULL,\n    type text NOT NULL,\n    key_index bigint NOT NULL,\n    quorum integer NOT NULL,\n    client_token text,\n    xpubs bytea[] NOT NULL\n);\n\n\n--\n-- Name: signers_key_index_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE signers_key_index_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: signers_key_index_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -\n--\n\nALTER SEQUENCE signers_key_index_seq OWNED BY signers.key_index;\n\n\n--\n-- Name: snapshots; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE snapshots (\n    height bigint NOT NULL,\n    data bytea NOT NULL,\n    created_at timestamp without time zone DEFAULT now()\n);\n\n\n--\n-- Name: submitted_txs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE submitted_txs (\n    tx_hash bytea NOT NULL,\n    height bigint NOT NULL,\n    submitted_at timestamp without time zone DEFAULT now() NOT NULL\n);\n\n\n--\n-- Name: txfeeds; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE txfeeds (\n    id text DEFAULT next_chain_id('cur'::text) NOT NULL,\n    alias text,\n    filter text,\n    after text,\n    client_token text\n);\n\n\n--\n-- Name: key_index; Type: DEFAULT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY signers ALTER COLUMN key_index SET DEFAULT nextval('signers_key_index_seq'::regclass);\n\n\n--\n-- Name: access_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY access_tokens\n    ADD CONSTRAINT access_tokens_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: account_control_programs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY account_control_programs\n    ADD CONSTRAINT account_control_programs_pkey PRIMARY KEY (control_program);\n\n\n--\n-- Name: account_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY accounts\n    ADD CONSTRAINT account_tags_pkey PRIMARY KEY (account_id);\n\n\n--\n-- Name: account_utxos_output_id_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY account_utxos\n    ADD CONSTRAINT account_utxos_output_id_key UNIQUE (output_id);\n\n\n--\n-- Name: account_utxos_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY account_utxos\n    ADD CONSTRAINT account_utxos_pkey PRIMARY KEY (tx_hash, index);\n\n\n--\n-- Name: accounts_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY accounts\n    ADD CONSTRAINT accounts_alias_key UNIQUE (alias);\n\n\n--\n-- Name: annotated_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_accounts\n    ADD CONSTRAINT annotated_accounts_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: annotated_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_assets\n    ADD CONSTRAINT annotated_assets_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: annotated_inputs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_inputs\n    ADD CONSTRAINT annotated_inputs_pkey PRIMARY KEY (tx_hash, index);\n\n\n--\n-- Name: annotated_outputs_output_id_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_outputs\n    ADD CONSTRAINT annotated_outputs_output_id_key UNIQUE (output_id);\n\n\n--\n-- Name: annotated_outputs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_outputs\n    ADD CONSTRAINT annotated_outputs_pkey PRIMARY KEY (block_height, tx_pos, output_index);\n\n\n--\n-- Name: annotated_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_txs\n    ADD CONSTRAINT annotated_txs_pkey PRIMARY KEY (block_height, tx_pos);\n\n\n--\n-- Name: asset_tags_asset_id_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY asset_tags\n    ADD CONSTRAINT asset_tags_asset_id_key UNIQUE (asset_id);\n\n\n--\n-- Name: assets_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY assets\n    ADD CONSTRAINT assets_alias_key UNIQUE (alias);\n\n\n--\n-- Name: assets_client_token_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY assets\n    ADD CONSTRAINT assets_client_token_key UNIQUE (client_token);\n\n\n--\n-- Name: assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY assets\n    ADD CONSTRAINT assets_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: block_processors_name_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY block_processors\n    ADD CONSTRAINT block_processors_name_key UNIQUE (name);\n\n\n--\n-- Name: blocks_height_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY blocks\n    ADD CONSTRAINT blocks_height_key UNIQUE (height);\n\n\n--\n-- Name: blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY blocks\n    ADD CONSTRAINT blocks_pkey PRIMARY KEY (block_hash);\n\n\n--\n-- Name: config_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY config\n    ADD CONSTRAINT config_pkey PRIMARY KEY (singleton);\n\n\n--\n-- Name: generator_pending_block_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY generator_pending_block\n    ADD CONSTRAINT generator_pending_block_pkey PRIMARY KEY (singleton);\n\n\n--\n-- Name: leader_singleton_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY leader\n    ADD CONSTRAINT leader_singleton_key UNIQUE (singleton);\n\n\n--\n-- Name: mockhsm_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY mockhsm\n    ADD CONSTRAINT mockhsm_alias_key UNIQUE (alias);\n\n\n--\n-- Name: mockhsm_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY mockhsm\n    ADD CONSTRAINT mockhsm_pkey PRIMARY KEY (pub);\n\n\n--\n-- Name: query_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY query_blocks\n    ADD CONSTRAINT query_blocks_pkey PRIMARY KEY (height);\n\n\n--\n-- Name: signers_client_token_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY signers\n    ADD CONSTRAINT signers_client_token_key UNIQUE (client_token);\n\n\n--\n-- Name: signers_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY signers\n    ADD CONSTRAINT signers_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: sort_id_index; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY mockhsm\n    ADD CONSTRAINT sort_id_index UNIQUE (sort_id);\n\n\n--\n-- Name: state_trees_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY snapshots\n    ADD CONSTRAINT state_trees_pkey PRIMARY KEY (height);\n\n\n--\n-- Name: submitted_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY submitted_txs\n    ADD CONSTRAINT submitted_txs_pkey PRIMARY KEY (tx_hash);\n\n\n--\n-- Name: txfeeds_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY txfeeds\n    ADD CONSTRAINT txfeeds_alias_key UNIQUE (alias);\n\n\n--\n-- Name: txfeeds_client_token_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY txfeeds\n    ADD CONSTRAINT txfeeds_client_token_key UNIQUE (client_token);\n\n\n--\n-- Name: txfeeds_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY txfeeds\n    ADD CONSTRAINT txfeeds_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: account_utxos_asset_id_account_id_confirmed_in_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX account_utxos_asset_id_account_id_confirmed_in_idx ON account_utxos USING btree (asset_id, account_id, confirmed_in);\n\n\n--\n-- Name: annotated_assets_sort_id; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_assets_sort_id ON annotated_assets USING btree (sort_id);\n\n\n--\n-- Name: annotated_outputs_timespan_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_outputs_timespan_idx ON annotated_outputs USING gist (timespan);\n\n\n--\n-- Name: annotated_txs_data_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_txs_data_idx ON annotated_txs USING gin (data jsonb_path_ops);\n\n\n--\n-- Name: assets_sort_id; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX assets_sort_id ON assets USING btree (sort_id);\n\n\n--\n-- Name: query_blocks_timestamp_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX query_blocks_timestamp_idx ON query_blocks USING btree (\"timestamp\");\n\n\n--\n-- Name: signed_blocks_block_height_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE UNIQUE INDEX signed_blocks_block_height_idx ON signed_blocks USING btree (block_height);\n\n\n--\n-- Name: signers_type_id_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX signers_type_id_idx ON signers USING btree (type, id);\n\n\n--\n-- PostgreSQL database dump complete\n--\n"},
	{Name: `2017-02-07.0.query.non-null-alias.sql`, SQL: `
		UPDATE annotated_assets SET alias = '' WHERE alias IS NULL;
		UPDATE annotated_accounts SET alias = '' WHERE alias IS NULL;
		ALTER TABLE annotated_assets ALTER COLUMN alias SET NOT NULL;
		ALTER TABLE annotated_accounts ALTER COLUMN alias SET NOT NULL;
	`},
	{Name: `2017-02-16.0.query.spent-output.sql`, SQL: `
		ALTER TABLE annotated_inputs
			ADD COLUMN spent_output_id bytea NOT NULL,
			ADD COLUMN spent_output jsonb;
	`},
	{Name: "2017-02-20.0.core.drop-account_utxo-index.sql", SQL: `
		ALTER TABLE account_utxos DROP CONSTRAINT account_utxos_pkey;
		ALTER TABLE account_utxos DROP CONSTRAINT account_utxos_output_id_key;
		ALTER TABLE account_utxos ADD CONSTRAINT account_utxos_pkey PRIMARY KEY (output_id);
		ALTER TABLE account_utxos DROP index;
	`},
	{Name: "2017-02-22.0.core.drop-account_utxo-txhash.sql", SQL: `
		ALTER TABLE account_utxos DROP tx_hash;
	`},
}
