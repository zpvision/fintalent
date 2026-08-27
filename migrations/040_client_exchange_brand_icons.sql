UPDATE client_exchange_dictionary_items
SET icon = CASE code
    WHEN 'ozon' THEN '/static/icons/client-exchange/marketplaces/ozon.svg'
    WHEN 'wildberries' THEN '/static/icons/client-exchange/marketplaces/wildberries.svg'
    WHEN 'yandex_market' THEN '/static/icons/client-exchange/marketplaces/yandex-market.svg'
    WHEN 'megamarket' THEN '/static/icons/client-exchange/marketplaces/megamarket.svg'
    WHEN 'lamoda' THEN '/static/icons/client-exchange/marketplaces/lamoda.svg'
    WHEN 'aliexpress' THEN '/static/icons/client-exchange/marketplaces/aliexpress.svg'
    WHEN 'avito' THEN '/static/icons/client-exchange/marketplaces/avito.svg'
    ELSE icon
END
WHERE kind = 'marketplace';

UPDATE client_exchange_dictionary_items
SET icon = CASE code
    WHEN 'diadoc' THEN '/static/icons/client-exchange/edo/diadoc.svg'
    WHEN 'saby' THEN '/static/icons/client-exchange/edo/saby.svg'
    WHEN 'taxcom' THEN '/static/icons/client-exchange/edo/taxcom.svg'
    WHEN 'astral' THEN '/static/icons/client-exchange/edo/astral.svg'
    WHEN 'sfera' THEN '/static/icons/client-exchange/edo/sfera.svg'
    WHEN 'edo_light' THEN '/static/icons/client-exchange/edo/edo-light.svg'
    WHEN '1c_edo' THEN '/static/icons/client-exchange/edo/1c-edo.svg'
    WHEN 'potok' THEN '/static/icons/client-exchange/edo/potok.svg'
    WHEN 'stek' THEN '/static/icons/client-exchange/edo/stek.svg'
    WHEN 'express' THEN '/static/icons/client-exchange/edo/express.svg'
    WHEN 'komita' THEN '/static/icons/client-exchange/edo/komita.svg'
    WHEN 'edisoft' THEN '/static/icons/client-exchange/edo/edisoft.svg'
    WHEN 'synerdocs' THEN '/static/icons/client-exchange/edo/synerdocs.svg'
    WHEN 'directum' THEN '/static/icons/client-exchange/edo/directum.svg'
    ELSE icon
END
WHERE kind = 'edo_provider';
