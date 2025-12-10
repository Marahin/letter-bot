-- Add notification_sent column to web_reservation table
ALTER TABLE web_reservation ADD COLUMN notification_sent BOOLEAN DEFAULT FALSE NOT NULL;
