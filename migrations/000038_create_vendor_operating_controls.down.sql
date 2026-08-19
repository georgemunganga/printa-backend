DROP TRIGGER IF EXISTS vendor_operating_reminder_immutable ON vendor_operating_reminders;
DROP FUNCTION IF EXISTS prevent_vendor_operating_reminder_mutation();
DROP TRIGGER IF EXISTS vendor_compliance_review_on_create ON vendors;
DROP FUNCTION IF EXISTS create_pending_vendor_compliance_review();

DROP TABLE IF EXISTS vendor_operating_reminders;
DROP TABLE IF EXISTS vendor_subscription_grace_periods;
DROP TABLE IF EXISTS vendor_compliance_reviews;
