-- Product-specific survey definitions are kept on the managed catalogue item so
-- the platform admin can revise customer questions without a frontend release.
UPDATE platform_products
SET attributes = COALESCE(attributes, '{}'::jsonb) || '{
  "printable_id": "business_card",
  "customization": {
    "version": 1,
    "title": "Customize your business cards",
    "intro": "Choose the card stock, sides, and finish for this pack.",
    "questions": [
      {"id":"sides","label":"Which sides should we print?","required":true,"options":[{"value":"single","label":"Front only","description":"Print on one side."},{"value":"double","label":"Front and back","description":"Print on both sides."}]},
      {"id":"stock","label":"How should the card feel?","required":true,"options":[{"value":"standard","label":"Standard card","description":"Classic business-card stock."},{"value":"thick","label":"Extra thick","description":"A heavier premium card."}]},
      {"id":"finish","label":"Which finish do you want?","required":true,"options":[{"value":"none","label":"No special finish","description":"Clean, natural card surface."},{"value":"matte","label":"Matte","description":"Smooth with low shine."},{"value":"gloss","label":"Gloss","description":"Bright, polished surface."}]}
    ]
  }
}'::jsonb
WHERE lower(name) LIKE '%business card%';

UPDATE platform_products
SET attributes = COALESCE(attributes, '{}'::jsonb) || '{
  "printable_id": "flyer_marketing",
  "customization": {
    "version": 1,
    "title": "Customize your flyers",
    "intro": "Choose the print coverage, colour, and paper finish.",
    "questions": [
      {"id":"sides","label":"Which sides should we print?","required":true,"options":[{"value":"single","label":"Front only","description":"A single-sided flyer."},{"value":"double","label":"Front and back","description":"Print on both sides."}]},
      {"id":"colour","label":"How should we print the colour?","required":true,"options":[{"value":"full_colour","label":"Full colour","description":"Vivid colour printing."},{"value":"black_white","label":"Black and white","description":"Simple monochrome printing."}]},
      {"id":"paper","label":"Which paper finish do you prefer?","required":true,"options":[{"value":"standard","label":"Standard","description":"Everyday flyer paper."},{"value":"gloss","label":"Gloss","description":"Smooth with a bright finish."},{"value":"matte","label":"Matte","description":"Smooth with low shine."}]}
    ]
  }
}'::jsonb
WHERE lower(name) LIKE '%flyer%';

UPDATE platform_products
SET attributes = COALESCE(attributes, '{}'::jsonb) || '{
  "printable_id": "s11-passport-photos",
  "customization": {
    "version": 1,
    "title": "Prepare your passport photos",
    "intro": "Confirm the document format and photo finish.",
    "questions": [
      {"id":"format","label":"What are the photos for?","required":true,"options":[{"value":"passport","label":"Passport","description":"Standard passport-photo format."},{"value":"visa","label":"Visa","description":"Photos for a visa application."},{"value":"other","label":"Another document","description":"Describe the required size below."}]},
      {"id":"finish","label":"Which photo finish do you prefer?","required":true,"options":[{"value":"gloss","label":"Glossy","description":"Bright traditional photo finish."},{"value":"matte","label":"Matte","description":"Low-glare photo finish."}]},
      {"id":"background","label":"Which background is required?","required":true,"options":[{"value":"white","label":"White","description":"Plain white background."},{"value":"other","label":"Another colour","description":"State the colour in the instructions."}]}
    ]
  }
}'::jsonb
WHERE lower(name) LIKE '%passport photo%';

UPDATE platform_products
SET attributes = COALESCE(attributes, '{}'::jsonb) || '{
  "printable_id": "s4-spiral-binding",
  "customization": {
    "version": 1,
    "title": "Customize spiral binding",
    "intro": "Tell us the document size, thickness, and cover preference.",
    "questions": [
      {"id":"size","label":"What size is the document?","required":true,"options":[{"value":"a4","label":"A4","description":"Standard office-document size."},{"value":"a5","label":"A5","description":"Compact half-A4 size."},{"value":"other","label":"Another size","description":"Add the dimensions below."}]},
      {"id":"pages","label":"About how many pages are there?","required":true,"options":[{"value":"up_to_50","label":"Up to 50","description":"A slim document."},{"value":"51_100","label":"51–100","description":"A medium document."},{"value":"over_100","label":"More than 100","description":"A thick document."}]},
      {"id":"cover","label":"Do you need a clear front cover?","required":true,"options":[{"value":"yes","label":"Yes","description":"Add a clear protective cover."},{"value":"no","label":"No","description":"Bind the supplied pages only."}]}
    ]
  }
}'::jsonb
WHERE lower(name) LIKE '%binding%spiral%' OR lower(name) LIKE '%spiral%binding%';

UPDATE platform_products
SET attributes = COALESCE(attributes, '{}'::jsonb) || '{
  "printable_id": "s1-a4",
  "customization": {
    "version": 1,
    "title": "Customize A4 lamination",
    "intro": "Choose the pouch thickness and surface finish.",
    "questions": [
      {"id":"thickness","label":"Which lamination thickness do you need?","required":true,"options":[{"value":"standard","label":"Standard","description":"Flexible everyday protection."},{"value":"heavy","label":"Heavy duty","description":"A firmer protective finish."}]},
      {"id":"finish","label":"Which surface finish do you prefer?","required":true,"options":[{"value":"gloss","label":"Gloss","description":"Clear with a bright shine."},{"value":"matte","label":"Matte","description":"Smooth with less glare."}]}
    ]
  }
}'::jsonb
WHERE lower(name) LIKE '%lamination%a4%' OR lower(name) LIKE '%a4%lamination%';
