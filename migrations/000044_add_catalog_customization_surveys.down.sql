UPDATE platform_products
SET attributes = (COALESCE(attributes, '{}'::jsonb) - 'customization' - 'printable_id')
WHERE lower(name) LIKE '%business card%'
   OR lower(name) LIKE '%flyer%'
   OR lower(name) LIKE '%passport photo%'
   OR lower(name) LIKE '%binding%spiral%'
   OR lower(name) LIKE '%spiral%binding%'
   OR lower(name) LIKE '%lamination%a4%'
   OR lower(name) LIKE '%a4%lamination%';
