-- Phase R coordination: finalized canonical document events are immutable.
--
-- Pending business-document events remain deletable inside a failed
-- transaction, and legacy state-less events remain compatible. Once a
-- business-document event is final (or has a finalization timestamp), DELETE
-- is rejected regardless of the caller's table DELETE privilege.

CREATE OR REPLACE FUNCTION reject_final_business_document_event_delete_017()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.aggregate = 'business_document'
       AND (
           OLD.payload->>'state' = 'final'
           OR OLD.finalized_at IS NOT NULL
       ) THEN
        RAISE EXCEPTION
            'finalized business document sync event % is immutable',
            OLD.event_id;
    END IF;
    RETURN OLD;
END;
$$;

DROP TRIGGER IF EXISTS sync_events_final_business_document_delete_017 ON sync_events;
CREATE TRIGGER sync_events_final_business_document_delete_017
BEFORE DELETE ON sync_events
FOR EACH ROW EXECUTE FUNCTION reject_final_business_document_event_delete_017();
