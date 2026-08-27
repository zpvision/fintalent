package main

import (
	"context"
	"embed"
	"net/http"

	"FinTalent/internal/clientexchange"
)

//go:embed migrations/039_client_exchange.sql migrations/042_client_exchange_icon_paths.sql
var clientExchangeMigrationFS embed.FS

func prepareClientExchangeDatabase(ctx context.Context) error {
	schema, err := clientExchangeMigrationFS.ReadFile("migrations/039_client_exchange.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(schema)); err != nil {
		return err
	}
	iconPaths, err := clientExchangeMigrationFS.ReadFile("migrations/042_client_exchange_icon_paths.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(iconPaths))
	return err
}

func registerClientExchangeRoutes() {
	handler := clientexchange.New(db, func(r *http.Request) (clientexchange.UserIdentity, error) {
		u, err := userFromRequest(r)
		if err != nil {
			return clientexchange.UserIdentity{}, err
		}
		return clientexchange.UserIdentity{ID: u.ID, FullName: u.FullName, Email: u.Email, Avatar: u.Avatar}, nil
	}, isAdmin)
	handler.Register(http.DefaultServeMux)
}

func prepareClientExchangeDemo(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `DO $$
	DECLARE seller BIGINT;
	BEGIN
		IF EXISTS(SELECT 1 FROM client_exchange_listings) THEN RETURN; END IF;
		SELECT id INTO seller FROM users ORDER BY id LIMIT 1;
		IF seller IS NULL THEN RETURN; END IF;
		INSERT INTO client_exchange_listings(seller_user_id,title,industry_id,employee_range_id,tax_system_id,revenue_range_id,accounting_state_id,transfer_reason_id,transfer_type_id,transfer_reason_comment,transfer_price,current_monthly_fee,operations_per_month,banks_count,has_vat,foreign_trade,bargain_allowed,region,city,client_since,status,match_percent,published_at)
		VALUES
		(seller,'Интернет-магазин',(SELECT id FROM client_exchange_dictionary_items WHERE kind='industry' AND code='online_stores'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='employee_range' AND code='6_10'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='tax_system' AND code='usn_profit'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='revenue_range' AND code='25_50m'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='accounting_state' AND code='good'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='transfer_reason' AND code='too_small'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='transfer_type' AND code='fixed'),'Оптимизируем клиентскую базу.',35000,18000,180,2,FALSE,FALSE,TRUE,'Москва','Москва','2022-03-01','active',NULL,NOW()-INTERVAL '2 days'),
		(seller,'Оптовая торговля',(SELECT id FROM client_exchange_dictionary_items WHERE kind='industry' AND code='wholesale'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='employee_range' AND code='11_20'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='tax_system' AND code='usn_income'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='revenue_range' AND code='50_100m'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='accounting_state' AND code='attention'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='transfer_reason' AND code='staff_shortage'),(SELECT id FROM client_exchange_dictionary_items WHERE kind='transfer_type' AND code='fixed'),'Передаём клиента команде с опытом ВЭД.',50000,25000,250,3,FALSE,TRUE,TRUE,'Санкт-Петербург','Санкт-Петербург','2021-09-01','active',NULL,NOW()-INTERVAL '6 days');
		INSERT INTO client_exchange_listing_options(listing_id,item_id,kind)
		SELECT l.id,d.id,d.kind FROM client_exchange_listings l CROSS JOIN client_exchange_dictionary_items d
		WHERE l.title='Интернет-магазин' AND ((d.kind='marketplace' AND d.code IN ('ozon','wildberries')) OR (d.kind='accounting_program' AND d.code='1c_accounting') OR (d.kind='edo_provider' AND d.code='diadoc')) ON CONFLICT DO NOTHING;
		INSERT INTO client_exchange_listing_options(listing_id,item_id,kind)
		SELECT l.id,d.id,d.kind FROM client_exchange_listings l CROSS JOIN client_exchange_dictionary_items d
		WHERE l.title='Оптовая торговля' AND ((d.kind='accounting_program' AND d.code='1c_complex') OR (d.kind='edo_provider' AND d.code='saby')) ON CONFLICT DO NOTHING;
	END $$;`)
	return err
}
