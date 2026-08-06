-- Canonical business-document event finalization.
--
-- A document projection needs the event identity before stock/GL rows can be
-- inserted, so the event is created as a pending envelope and finalized after
-- those projections have been read back. This migration makes that one
-- pending -> final transition explicit and rejects every later mutation.
-- Legacy non-state payloads remain readable and are not rewritten.

ALTER TABLE sync_events
    ADD COLUMN IF NOT EXISTS finalized_at timestamptz;

-- Existing stateful final events were written by the application before this
-- integrity boundary existed. Preserve their canonical payload and record
-- the already-observed acceptance time rather than rewriting their contents.
UPDATE sync_events
SET finalized_at = accepted_at
WHERE aggregate = 'business_document'
  AND payload->>'state' = 'final'
  AND finalized_at IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sync_events_business_document_final_payload_015'
    ) THEN
        ALTER TABLE sync_events
            ADD CONSTRAINT sync_events_business_document_final_payload_015
            CHECK (
                aggregate <> 'business_document'
                OR payload->>'state' IS NULL
                OR (
                    payload->>'state' = 'pending'
                    AND finalized_at IS NULL
                )
                OR (
                    payload->>'state' = 'final'
                    AND finalized_at IS NOT NULL
                    AND payload ? 'eventId'
                    AND payload ? 'document'
                )
            );
    END IF;
END $$;

CREATE OR REPLACE FUNCTION guard_business_document_final_payload_015()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.aggregate = 'business_document'
           AND NEW.payload->>'state' = 'final' THEN
            IF NEW.payload->>'eventId' IS DISTINCT FROM NEW.event_id::text
               OR NOT (NEW.payload ? 'document') THEN
                RAISE EXCEPTION 'business document final payload is not canonical';
            END IF;
            NEW.finalized_at := COALESCE(NEW.finalized_at, now());
        ELSE
            NEW.finalized_at := NULL;
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.aggregate <> 'business_document' THEN
        RETURN NEW;
    END IF;

    IF OLD.payload->>'state' = 'pending'
       AND NEW.payload->>'state' = 'final'
       AND NEW.payload->>'eventId' = NEW.event_id::text
       AND NEW.payload ? 'document'
       AND NEW.event_id = OLD.event_id
       AND NEW.tenant_id = OLD.tenant_id
       AND NEW.branch_id = OLD.branch_id
       AND NEW.aggregate = OLD.aggregate
       AND NEW.aggregate_id = OLD.aggregate_id
       AND NEW.idempotency_key = OLD.idempotency_key
       AND NEW.schema_version = OLD.schema_version
       AND NEW.counter_id IS NOT DISTINCT FROM OLD.counter_id
       AND NEW.operator_id IS NOT DISTINCT FROM OLD.operator_id
       AND NEW.occurred_at = OLD.occurred_at
       AND NEW.accepted_at = OLD.accepted_at THEN
        NEW.finalized_at := COALESCE(NEW.finalized_at, now());
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'business document sync event is immutable after finalization';
END;
$$;

DROP TRIGGER IF EXISTS sync_events_final_payload_transition_015 ON sync_events;
CREATE TRIGGER sync_events_final_payload_transition_015
BEFORE INSERT OR UPDATE ON sync_events
FOR EACH ROW EXECUTE FUNCTION guard_business_document_final_payload_015();

CREATE INDEX IF NOT EXISTS idx_sync_events_finalized
    ON sync_events (tenant_id, branch_id, finalized_at DESC)
    WHERE finalized_at IS NOT NULL;
