-- Add notification_sent column to web_reservation table
ALTER TABLE web_reservation ADD COLUMN notification_sent BOOLEAN DEFAULT FALSE NOT NULL;

-- Add partial index for optimized notification polling
CREATE INDEX web_reservation_start_at_notification_unsent_idx
  ON web_reservation (start_at)
  WHERE notification_sent = FALSE;