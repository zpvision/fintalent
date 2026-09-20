package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/027_profimarket.sql migrations/028_profimarket_demo.sql migrations/029_profimarket_card_builder.sql migrations/030_profimarket_section_images.sql migrations/031_profimarket_crm_dictionary.sql migrations/032_profimarket_feature_colors.sql migrations/033_profimarket_bonus_style.sql migrations/034_profimarket_block_styles.sql migrations/035_profimarket_right_block.sql migrations/036_profimarket_implementation.sql migrations/037_profimarket_section_appearance.sql migrations/049_profimarket_platform_icons.sql migrations/050_profimarket_how_it_works.sql migrations/051_profimarket_product_types.sql migrations/052_profimarket_product_demo.sql migrations/055_profimarket_order_notifications.sql migrations/056_profimarket_purchase_snapshot.sql migrations/057_profimarket_onec_configurations.sql migrations/059_profimarket_compatibility.sql
var profiMarketMigrationFS embed.FS

type profiMedia struct {
	ID        int64  `json:"id,omitempty"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
	IsPreview bool   `json:"is_preview"`
}
type profiFeature struct {
	ID              int64  `json:"id,omitempty"`
	Icon            string `json:"icon"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	SortOrder       int    `json:"sort_order"`
	TextColor       string `json:"text_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
}
type profiItem struct {
	ID          int64  `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}
type profiSection struct {
	ID             int64       `json:"id,omitempty"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	ImageURL       string      `json:"image_url,omitempty"`
	IconImageURL   string      `json:"icon_image_url,omitempty"`
	NumberingColor string      `json:"numbering_color,omitempty"`
	SortOrder      int         `json:"sort_order"`
	Items          []profiItem `json:"items"`
}
type profiDictionaryValue struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}
type profiSolution struct {
	ID                     int64                  `json:"id"`
	AuthorUserID           int64                  `json:"author_user_id"`
	AuthorName             string                 `json:"author_name"`
	AuthorAvatar           string                 `json:"author_avatar,omitempty"`
	Type                   string                 `json:"type"`
	Status                 string                 `json:"status"`
	Title                  string                 `json:"title"`
	Slug                   string                 `json:"slug"`
	ShortDescription       string                 `json:"short_description"`
	Description            string                 `json:"description"`
	CoverImage             string                 `json:"cover_image"`
	Price                  float64                `json:"price"`
	OldPrice               *float64               `json:"old_price,omitempty"`
	Currency               string                 `json:"currency"`
	PricingType            string                 `json:"pricing_type"`
	TrialDays              int                    `json:"trial_days"`
	DeliveryType           string                 `json:"delivery_type"`
	ExternalURL            string                 `json:"external_url,omitempty"`
	Tags                   []string               `json:"tags"`
	Topics                 []string               `json:"topics"`
	Audiences              []string               `json:"audiences"`
	IsFeatured             bool                   `json:"is_featured"`
	IsNew                  bool                   `json:"is_new"`
	ViewsCount             int                    `json:"views_count"`
	PurchasesCount         int                    `json:"purchases_count"`
	FavoritesCount         int                    `json:"favorites_count"`
	Rating                 float64                `json:"rating"`
	ReviewCount            int                    `json:"review_count"`
	IsFavorite             bool                   `json:"is_favorite"`
	IsAuthor               bool                   `json:"is_author"`
	PublishedAt            *time.Time             `json:"published_at,omitempty"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
	Sections               []profiSection         `json:"sections"`
	AccessFeatures         []profiFeature         `json:"access_features"`
	AIFeatures             []profiFeature         `json:"ai_features"`
	HowItWorks             []profiFeature         `json:"how_it_works"`
	Media                  []profiMedia           `json:"media"`
	CRMs                   []profiDictionaryValue `json:"crms"`
	Platforms              []profiDictionaryValue `json:"platforms"`
	KeyMetrics             []profiFeature         `json:"key_metrics"`
	Bonuses                []profiFeature         `json:"bonuses"`
	BonusStyle             string                 `json:"bonus_style"`
	MetricStyle            string                 `json:"metric_style"`
	AccessStyle            string                 `json:"access_style"`
	RightBlockTitle        string                 `json:"right_block_title"`
	ImplementationTitle    string                 `json:"implementation_title"`
	ImplementationSubtitle string                 `json:"implementation_subtitle"`
	PurchaseButtonCode     string                 `json:"purchase_button_code"`
	PurchaseButtonLabel    string                 `json:"purchase_button_label"`
	ProductData            map[string]any         `json:"product_data"`
}
type profiSolutionInput struct {
	Type                   string         `json:"type"`
	Title                  string         `json:"title"`
	ShortDescription       string         `json:"short_description"`
	Description            string         `json:"description"`
	CoverImage             string         `json:"cover_image"`
	Price                  float64        `json:"price"`
	OldPrice               *float64       `json:"old_price"`
	Currency               string         `json:"currency"`
	PricingType            string         `json:"pricing_type"`
	TrialDays              int            `json:"trial_days"`
	DeliveryType           string         `json:"delivery_type"`
	ExternalURL            string         `json:"external_url"`
	Tags                   []string       `json:"tags"`
	Topics                 []string       `json:"topics"`
	Audiences              []string       `json:"audiences"`
	Sections               []profiSection `json:"sections"`
	AccessFeatures         []profiFeature `json:"access_features"`
	AIFeatures             []profiFeature `json:"ai_features"`
	HowItWorks             []profiFeature `json:"how_it_works"`
	Media                  []profiMedia   `json:"media"`
	CRMIDs                 []int64        `json:"crm_ids"`
	PlatformIDs            []int64        `json:"platform_ids"`
	KeyMetrics             []profiFeature `json:"key_metrics"`
	Bonuses                []profiFeature `json:"bonuses"`
	BonusStyle             string         `json:"bonus_style"`
	MetricStyle            string         `json:"metric_style"`
	AccessStyle            string         `json:"access_style"`
	RightBlockTitle        string         `json:"right_block_title"`
	ImplementationTitle    string         `json:"implementation_title"`
	ImplementationSubtitle string         `json:"implementation_subtitle"`
	PurchaseButtonCode     string         `json:"purchase_button_code"`
	ProductData            map[string]any `json:"product_data"`
}

func prepareProfiMarketDatabase(ctx context.Context) error {
	schema, err := profiMarketMigrationFS.ReadFile("migrations/027_profimarket.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("схема ПрофиМаркета: %w", err)
	}
	builder, err := profiMarketMigrationFS.ReadFile("migrations/029_profimarket_card_builder.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(builder)); err != nil {
		return fmt.Errorf("конструктор карточек ПрофиМаркета: %w", err)
	}
	sectionImages, err := profiMarketMigrationFS.ReadFile("migrations/030_profimarket_section_images.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(sectionImages)); err != nil {
		return fmt.Errorf("изображения групп ПрофиМаркета: %w", err)
	}
	crmDictionary, err := profiMarketMigrationFS.ReadFile("migrations/031_profimarket_crm_dictionary.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(crmDictionary)); err != nil {
		return fmt.Errorf("справочник CRM ПрофиМаркета: %w", err)
	}
	featureColors, err := profiMarketMigrationFS.ReadFile("migrations/032_profimarket_feature_colors.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(featureColors)); err != nil {
		return fmt.Errorf("цвета карточек ПрофиМаркета: %w", err)
	}
	bonusStyle, err := profiMarketMigrationFS.ReadFile("migrations/033_profimarket_bonus_style.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(bonusStyle)); err != nil {
		return fmt.Errorf("стили бонусов ПрофиМаркета: %w", err)
	}
	blockStyles, err := profiMarketMigrationFS.ReadFile("migrations/034_profimarket_block_styles.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(blockStyles)); err != nil {
		return fmt.Errorf("стили блоков ПрофиМаркета: %w", err)
	}
	rightBlock, err := profiMarketMigrationFS.ReadFile("migrations/035_profimarket_right_block.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(rightBlock)); err != nil {
		return fmt.Errorf("правый блок ПрофиМаркета: %w", err)
	}
	implementation, err := profiMarketMigrationFS.ReadFile("migrations/036_profimarket_implementation.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(implementation)); err != nil {
		return fmt.Errorf("настройки внедрения ПрофиМаркета: %w", err)
	}
	sectionAppearance, err := profiMarketMigrationFS.ReadFile("migrations/037_profimarket_section_appearance.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(sectionAppearance)); err != nil {
		return fmt.Errorf("оформление групп ПрофиМаркета: %w", err)
	}
	platformIcons, err := profiMarketMigrationFS.ReadFile("migrations/049_profimarket_platform_icons.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(platformIcons)); err != nil {
		return fmt.Errorf("иконки платформ ПрофиМаркета: %w", err)
	}
	howItWorks, err := profiMarketMigrationFS.ReadFile("migrations/050_profimarket_how_it_works.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(howItWorks)); err != nil {
		return fmt.Errorf("шаги работы ИИ-ассистента: %w", err)
	}
	productTypes, err := profiMarketMigrationFS.ReadFile("migrations/051_profimarket_product_types.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(productTypes)); err != nil {
		return fmt.Errorf("типы продуктов ПрофиМаркета: %w", err)
	}
	orderNotifications, err := profiMarketMigrationFS.ReadFile("migrations/055_profimarket_order_notifications.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(orderNotifications)); err != nil {
		return fmt.Errorf("уведомления о заказах ПрофиМаркета: %w", err)
	}
	purchaseSnapshot, err := profiMarketMigrationFS.ReadFile("migrations/056_profimarket_purchase_snapshot.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(purchaseSnapshot)); err != nil {
		return fmt.Errorf("снимки покупок ПрофиМаркета: %w", err)
	}
	onecConfigurations, err := profiMarketMigrationFS.ReadFile("migrations/057_profimarket_onec_configurations.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(onecConfigurations)); err != nil {
		return fmt.Errorf("конфигурации 1С ПрофиМаркета: %w", err)
	}
	compatibility, err := profiMarketMigrationFS.ReadFile("migrations/059_profimarket_compatibility.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(compatibility)); err != nil {
		return fmt.Errorf("совместимость автоматизаций ПрофиМаркета: %w", err)
	}
	if err = syncProfiMarketCRMs(ctx); err != nil {
		return fmt.Errorf("синхронизация CRM ПрофиМаркета: %w", err)
	}
	return nil
}

func syncProfiMarketCRMs(ctx context.Context) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE profimarket_crm p SET source_dictionary_item_id=i.id,name=i.value,description=i.comment,icon=i.icon,sort_order=i.sort_order,active=i.active AND i.deleted_at IS NULL FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id AND d.alias='crm' WHERE lower(p.name)=lower(i.value) AND p.source_dictionary_item_id IS NULL AND p.code<>'other'`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE profimarket_crm p SET name=i.value,description=i.comment,icon=i.icon,active=i.active AND i.deleted_at IS NULL,sort_order=i.sort_order FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id AND d.alias='crm' WHERE p.source_dictionary_item_id=i.id`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO profimarket_crm(code,name,description,icon,active,sort_order,source_dictionary_item_id) SELECT 'dictionary-'||i.id,i.value,i.comment,i.icon,i.active AND i.deleted_at IS NULL,i.sort_order,i.id FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE d.alias='crm' AND NOT EXISTS(SELECT 1 FROM profimarket_crm p WHERE p.source_dictionary_item_id=i.id)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE profimarket_crm SET active=FALSE WHERE source_dictionary_item_id IS NULL AND code<>'other'`); err != nil {
		return err
	}
	return tx.Commit()
}
func prepareProfiMarketDemo(ctx context.Context) error {
	schema, err := profiMarketMigrationFS.ReadFile("migrations/028_profimarket_demo.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("демо ПрофиМаркета: %w", err)
	}
	products, err := profiMarketMigrationFS.ReadFile("migrations/052_profimarket_product_demo.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(products)); err != nil {
		return fmt.Errorf("демо продуктов ПрофиМаркета: %w", err)
	}
	return nil
}

func registerProfiMarketRoutes() {
	http.HandleFunc("/profimarket", serveFrontendPage("static/profimarket.html"))
	http.HandleFunc("/profimarket/create", serveFrontendPage("static/profimarket-create.html"))
	http.HandleFunc("/profimarket/regulation/edit", serveFrontendPage("static/profimarket-regulation-edit.html"))
	http.HandleFunc("/profimarket/product/edit", serveFrontendPage("static/profimarket-create.html"))
	http.HandleFunc("/profimarket/my", serveFrontendPage("static/profimarket-my.html"))
	http.HandleFunc("/profimarket/solution/", serveFrontendPage("static/profimarket-detail.html"))
	http.HandleFunc("/api/profimarket", profiMarketCollectionAPI)
	http.HandleFunc("/api/profimarket/meta", profiMarketMetaAPI)
	http.HandleFunc("/api/profimarket/upload", profiMarketUploadAPI)
	http.HandleFunc("/api/profimarket/my-solutions", profiMarketMySolutionsAPI)
	http.HandleFunc("/api/profimarket/my-purchases", profiMarketMyPurchasesAPI)
	http.HandleFunc("/api/profimarket/my-orders", profiMarketMyOrdersAPI)
	http.HandleFunc("/api/profimarket/reviews", profiMarketReviewsAPI)
	http.HandleFunc("/api/profimarket/solution/", profiMarketSolutionAPI)
}

func profiMarketReviewsAPI(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		writeJSON(w, http.StatusServiceUnavailable, "База данных недоступна")
		return
	}
	if r.Method == http.MethodGet {
		solutionID, err := strconv.ParseInt(r.URL.Query().Get("solution_id"), 10, 64)
		if err != nil || solutionID <= 0 {
			writeJSON(w, http.StatusBadRequest, "Некорректное решение")
			return
		}
		u := profiCurrentUser(r)
		userID := int64(0)
		if u != nil {
			userID = u.ID
		}
		rows, err := db.QueryContext(r.Context(), `SELECT rv.id,rv.rating,rv.comment,rv.created_at,u.full_name,COALESCE(u.avatar_url,''),rv.user_id=$2
			FROM profimarket_reviews rv JOIN users u ON u.id=rv.user_id
			WHERE rv.solution_id=$1 ORDER BY rv.created_at DESC,rv.id DESC`, solutionID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить отзывы")
			return
		}
		defer rows.Close()
		reviews := []map[string]any{}
		for rows.Next() {
			var id int64
			var rating int
			var comment, name, avatar string
			var created time.Time
			var mine bool
			if err = rows.Scan(&id, &rating, &comment, &created, &name, &avatar, &mine); err != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить отзывы")
				return
			}
			reviews = append(reviews, map[string]any{"id": id, "rating": rating, "comment": comment, "created_at": created, "author_name": name, "author_avatar": avatar, "is_mine": mine})
		}
		if err = rows.Err(); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить отзывы")
			return
		}
		canReview := false
		if userID > 0 {
			err = db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM profimarket_purchases p JOIN profimarket_solutions s ON s.id=p.solution_id WHERE p.solution_id=$1 AND p.buyer_user_id=$2 AND p.status='COMPLETED' AND s.author_user_id<>$2)`, solutionID, userID).Scan(&canReview)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось проверить покупку")
				return
			}
		}
		profiRespond(w, http.StatusOK, map[string]any{"reviews": reviews, "can_review": canReview})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	u, ok := profiRequireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		SolutionID int64  `json:"solution_id"`
		Rating     int    `json:"rating"`
		Comment    string `json:"comment"`
	}
	if !profiDecode(w, r, &input) {
		return
	}
	input.Comment = strings.TrimSpace(input.Comment)
	if input.SolutionID <= 0 || input.Rating < 1 || input.Rating > 5 || len([]rune(input.Comment)) > 2000 {
		writeJSON(w, http.StatusBadRequest, "Укажите оценку от 1 до 5 и комментарий до 2000 символов")
		return
	}
	var purchaseID int64
	err := db.QueryRowContext(r.Context(), `SELECT p.id FROM profimarket_purchases p JOIN profimarket_solutions s ON s.id=p.solution_id
		WHERE p.solution_id=$1 AND p.buyer_user_id=$2 AND p.status='COMPLETED' AND s.author_user_id<>$2 ORDER BY p.created_at DESC,p.id DESC LIMIT 1`, input.SolutionID, u.ID).Scan(&purchaseID)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusForbidden, "Отзыв доступен после покупки или оформления пробного периода")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось проверить покупку")
		return
	}
	_, err = db.ExecContext(r.Context(), `INSERT INTO profimarket_reviews(solution_id,purchase_id,user_id,rating,comment) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(solution_id,user_id) DO UPDATE SET purchase_id=EXCLUDED.purchase_id,rating=EXCLUDED.rating,comment=EXCLUDED.comment,updated_at=NOW()`, input.SolutionID, purchaseID, u.ID, input.Rating, input.Comment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить отзыв")
		return
	}
	profiRespond(w, http.StatusOK, map[string]string{"message": "Спасибо! Ваш отзыв опубликован."})
}

func profiRespond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func profiDecode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		writeJSON(w, 400, "Некорректные данные")
		return false
	}
	return true
}
func profiCurrentUser(r *http.Request) *user {
	u, err := userFromRequest(r)
	if err != nil {
		return nil
	}
	return u
}
func profiRequireUser(w http.ResponseWriter, r *http.Request) (*user, bool) {
	u := profiCurrentUser(r)
	if u == nil {
		writeJSON(w, 401, "Требуется авторизация")
		return nil, false
	}
	return u, true
}
func cleanStrings(values []string) []string {
	out, seen := []string{}, map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && len([]rune(value)) <= 120 && !seen[strings.ToLower(value)] {
			seen[strings.ToLower(value)] = true
			out = append(out, value)
		}
		if len(out) == 20 {
			break
		}
	}
	return out
}
func stringArrayJSON(values []string) string {
	b, _ := json.Marshal(cleanStrings(values))
	return string(b)
}
func decodeStringArray(raw []byte) []string {
	values := []string{}
	_ = json.Unmarshal(raw, &values)
	return values
}
func validateProfiInput(input *profiSolutionInput, publishing bool) error {
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	validTypes := map[string]bool{"REGULATION": true, "AI_ASSISTANT": true, "AUTOMATION": true, "INSTRUCTION": true, "ONEC_INTEGRATION": true, "TEMPLATE": true, "CHECKLIST": true}
	if !validTypes[input.Type] {
		return errors.New("выберите тип решения")
	}
	input.Title = strings.TrimSpace(input.Title)
	input.ShortDescription = strings.TrimSpace(input.ShortDescription)
	input.Description = strings.TrimSpace(input.Description)
	if publishing && len([]rune(input.Title)) < 8 {
		return errors.New("название должно содержать не менее 8 символов")
	}
	if len([]rune(input.Title)) > 240 || len([]rune(input.ShortDescription)) > 1200 || len([]rune(input.Description)) > 20000 {
		return errors.New("одно из текстовых полей слишком длинное")
	}
	if input.Price < 0 || input.TrialDays < 0 {
		return errors.New("цена и пробный период не могут быть отрицательными")
	}
	if !map[string]bool{"ONE_TIME": true, "MONTHLY": true, "YEARLY": true, "FREE": true}[input.PricingType] {
		input.PricingType = "ONE_TIME"
	}
	if input.PricingType == "FREE" {
		input.Price = 0
	}
	if !map[string]bool{"LINK": true, "MANUAL": true}[input.DeliveryType] {
		input.DeliveryType = "MANUAL"
	}
	input.Currency = "RUB"
	if !map[string]bool{"metrics-default": true, "access-default": true, "amber": true, "violet": true, "ocean": true, "mint": true, "rose": true, "graphite": true, "sky": true, "terracotta": true, "lavender": true, "aurora": true, "sand": true}[input.BonusStyle] {
		input.BonusStyle = "amber"
	}
	validBlockStyles := map[string]bool{"metrics-default": true, "access-default": true, "amber": true, "violet": true, "ocean": true, "mint": true, "rose": true, "graphite": true, "sky": true, "terracotta": true, "lavender": true, "aurora": true, "sand": true}
	if !validBlockStyles[input.MetricStyle] {
		input.MetricStyle = "metrics-default"
	}
	if !validBlockStyles[input.AccessStyle] {
		input.AccessStyle = "access-default"
	}
	input.RightBlockTitle = strings.TrimSpace(input.RightBlockTitle)
	if len([]rune(input.RightBlockTitle)) > 120 {
		return errors.New("название правого блока слишком длинное")
	}
	input.ImplementationTitle = strings.TrimSpace(input.ImplementationTitle)
	input.ImplementationSubtitle = strings.TrimSpace(input.ImplementationSubtitle)
	if input.ImplementationTitle == "" {
		input.ImplementationTitle = "Регламенты навсегда в вашей CRM"
	}
	if input.ImplementationSubtitle == "" {
		input.ImplementationSubtitle = "Доступ получаете вы, они остаются у вас"
	}
	if len([]rune(input.ImplementationTitle)) > 240 || len([]rune(input.ImplementationSubtitle)) > 500 {
		return errors.New("текст блока внедрения слишком длинный")
	}
	input.PurchaseButtonCode = strings.TrimSpace(input.PurchaseButtonCode)
	if input.PurchaseButtonCode == "" {
		input.PurchaseButtonCode = "buy_and_implement"
	}
	input.Tags, input.Topics, input.Audiences = cleanStrings(input.Tags), cleanStrings(input.Topics), cleanStrings(input.Audiences)
	return nil
}

func profiMarketCollectionAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profiMarketList(w, r, false, 0)
	case http.MethodPost:
		u, ok := profiRequireUser(w, r)
		if !ok {
			return
		}
		var input profiSolutionInput
		if !profiDecode(w, r, &input) {
			return
		}
		if err := validateProfiInput(&input, false); err != nil {
			writeJSON(w, 400, err.Error())
			return
		}
		suffix := make([]byte, 5)
		_, _ = rand.Read(suffix)
		slug := slugify(input.Title)
		if slug == "" {
			slug = "novoe-reshenie"
		}
		slug += "-" + hex.EncodeToString(suffix)
		var id int64
		err := db.QueryRowContext(r.Context(), `INSERT INTO profimarket_solutions(author_user_id,type,title,slug,pricing_type,delivery_type,currency) VALUES($1,$2,$3,$4,$5,$6,'RUB') RETURNING id`, u.ID, input.Type, input.Title, slug, input.PricingType, input.DeliveryType).Scan(&id)
		if err != nil {
			writeJSON(w, 500, "Не удалось создать решение")
			return
		}
		if err = saveProfiSolution(r.Context(), id, u.ID, input); err != nil {
			writeJSON(w, 400, err.Error())
			return
		}
		solution, _ := loadProfiSolution(r.Context(), strconv.FormatInt(id, 10), u)
		profiRespond(w, 201, solution)
	default:
		writeJSON(w, 405, "Метод не поддерживается")
	}
}

func profiMarketList(w http.ResponseWriter, r *http.Request, own bool, userID int64) {
	where := "s.status='PUBLISHED' AND s.deleted_at IS NULL"
	args := []any{}
	if own {
		where = "s.author_user_id=$1 AND s.deleted_at IS NULL"
		args = append(args, userID)
	}
	if !own {
		if value := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type"))); map[string]bool{"REGULATION": true, "AI_ASSISTANT": true, "AUTOMATION": true, "INSTRUCTION": true, "ONEC_INTEGRATION": true, "TEMPLATE": true, "CHECKLIST": true}[value] {
			args = append(args, value)
			where += fmt.Sprintf(" AND s.type=$%d", len(args))
		}
		if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
			args = append(args, q)
			where += fmt.Sprintf(" AND (s.title ILIKE '%%'||$%d||'%%' OR s.short_description ILIKE '%%'||$%d||'%%' OR $%d=ANY(s.tags))", len(args), len(args), len(args))
		}
		if r.URL.Query().Get("free") == "true" {
			where += " AND (s.pricing_type='FREE' OR s.price=0)"
		}
	}
	order := "s.is_featured DESC,s.published_at DESC NULLS LAST,s.created_at DESC"
	switch r.URL.Query().Get("sort") {
	case "popular":
		order = "purchases_count DESC,rating DESC,s.published_at DESC"
	case "new":
		order = "s.published_at DESC NULLS LAST"
	}
	query := `SELECT s.id,s.author_user_id,u.full_name,COALESCE(u.avatar_url,''),s.type,s.status,s.title,s.slug,s.short_description,s.description,s.cover_image,s.price,s.old_price,s.currency,s.pricing_type,s.trial_days,s.delivery_type,s.external_url,array_to_json(s.tags),array_to_json(s.topics),array_to_json(s.audiences),s.is_featured,s.is_new,s.views_count,s.published_at,s.created_at,s.updated_at,
		(SELECT COUNT(*) FROM profimarket_purchases p WHERE p.solution_id=s.id AND p.status='COMPLETED') purchases_count,
		(SELECT COUNT(*) FROM profimarket_favorites f WHERE f.solution_id=s.id) favorites_count,
		COALESCE((SELECT AVG(rv.rating) FROM profimarket_reviews rv WHERE rv.solution_id=s.id),0) rating,
		(SELECT COUNT(*) FROM profimarket_reviews rv WHERE rv.solution_id=s.id) review_count
		FROM profimarket_solutions s JOIN users u ON u.id=s.author_user_id WHERE ` + where + ` ORDER BY ` + order + ` LIMIT 100`
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить ПрофиМаркет")
		return
	}
	defer rows.Close()
	items := []profiSolution{}
	for rows.Next() {
		var x profiSolution
		var old sql.NullFloat64
		var published sql.NullTime
		var tags, topics, audiences []byte
		if err = rows.Scan(&x.ID, &x.AuthorUserID, &x.AuthorName, &x.AuthorAvatar, &x.Type, &x.Status, &x.Title, &x.Slug, &x.ShortDescription, &x.Description, &x.CoverImage, &x.Price, &old, &x.Currency, &x.PricingType, &x.TrialDays, &x.DeliveryType, &x.ExternalURL, &tags, &topics, &audiences, &x.IsFeatured, &x.IsNew, &x.ViewsCount, &published, &x.CreatedAt, &x.UpdatedAt, &x.PurchasesCount, &x.FavoritesCount, &x.Rating, &x.ReviewCount); err != nil {
			continue
		}
		if old.Valid {
			x.OldPrice = &old.Float64
		}
		if published.Valid {
			x.PublishedAt = &published.Time
		}
		x.Tags = decodeStringArray(tags)
		x.Topics = decodeStringArray(topics)
		x.Audiences = decodeStringArray(audiences)
		items = append(items, x)
	}
	profiRespond(w, 200, map[string]any{"items": items, "total": len(items)})
}

func profiMarketSolutionAPI(w http.ResponseWriter, r *http.Request) {
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/profimarket/solution/"), "/")
	parts := strings.Split(tail, "/")
	if tail == "" {
		writeJSON(w, 404, "Решение не найдено")
		return
	}
	key, action := parts[0], ""
	if len(parts) > 1 {
		action = parts[1]
	}
	u := profiCurrentUser(r)
	if action == "" {
		switch r.Method {
		case http.MethodGet:
			x, err := loadProfiSolution(r.Context(), key, u)
			if err != nil {
				writeJSON(w, 404, "Решение не найдено")
				return
			}
			if x.Status != "PUBLISHED" && (u == nil || x.AuthorUserID != u.ID) && !isAdmin(r) {
				writeJSON(w, 404, "Решение не найдено")
				return
			}
			if x.Status == "PUBLISHED" {
				_, _ = db.ExecContext(r.Context(), `UPDATE profimarket_solutions SET views_count=views_count+1 WHERE id=$1`, x.ID)
			}
			profiRespond(w, 200, x)
		case http.MethodPut:
			if u == nil {
				writeJSON(w, 401, "Требуется авторизация")
				return
			}
			id, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				writeJSON(w, 400, "Некорректный id")
				return
			}
			var input profiSolutionInput
			if !profiDecode(w, r, &input) {
				return
			}
			if err = validateProfiInput(&input, false); err != nil {
				writeJSON(w, 400, err.Error())
				return
			}
			if err = saveProfiSolution(r.Context(), id, u.ID, input); err != nil {
				writeJSON(w, 403, err.Error())
				return
			}
			x, _ := loadProfiSolution(r.Context(), key, u)
			profiRespond(w, 200, x)
		case http.MethodDelete:
			id, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				writeJSON(w, 400, "Некорректный id")
				return
			}
			if u == nil && !isAdmin(r) {
				writeJSON(w, 401, "Требуется авторизация")
				return
			}
			res, e := db.ExecContext(r.Context(), `UPDATE profimarket_solutions SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND ($2 OR author_user_id=$3)`, id, isAdmin(r), func() int64 {
				if u != nil {
					return u.ID
				}
				return 0
			}())
			if e != nil {
				writeJSON(w, 500, "Не удалось удалить решение")
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				writeJSON(w, 403, "Удалить можно только своё решение")
				return
			}
			writeJSON(w, 200, "Решение удалено")
		default:
			writeJSON(w, 405, "Метод не поддерживается")
		}
		return
	}
	id, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		writeJSON(w, 400, "Некорректный id")
		return
	}
	switch action {
	case "publish":
		if r.Method != http.MethodPost || u == nil {
			writeJSON(w, 401, "Требуется авторизация")
			return
		}
		x, e := loadProfiSolution(r.Context(), key, u)
		if e != nil || x.AuthorUserID != u.ID {
			writeJSON(w, 403, "Недостаточно прав")
			return
		}
		input := solutionToInput(x)
		if e = validateProfiInput(&input, true); e != nil {
			writeJSON(w, 400, e.Error())
			return
		}
		if x.Type == "REGULATION" && len(x.Sections) == 0 {
			writeJSON(w, 400, "Добавьте состав пакета")
			return
		}
		if x.Type == "AI_ASSISTANT" && len(x.AIFeatures) == 0 {
			writeJSON(w, 400, "Добавьте возможности помощника")
			return
		}
		_, e = db.ExecContext(r.Context(), `UPDATE profimarket_solutions SET status='PUBLISHED',published_at=COALESCE(published_at,NOW()),updated_at=NOW() WHERE id=$1 AND author_user_id=$2`, id, u.ID)
		if e != nil {
			writeJSON(w, 500, "Не удалось опубликовать")
			return
		}
		writeJSON(w, 200, "Решение опубликовано")
	case "unpublish":
		if r.Method != http.MethodPost || u == nil {
			writeJSON(w, 401, "Требуется авторизация")
			return
		}
		_, err = db.ExecContext(r.Context(), `UPDATE profimarket_solutions SET status='ARCHIVED',updated_at=NOW() WHERE id=$1 AND author_user_id=$2`, id, u.ID)
		if err != nil {
			writeJSON(w, 500, "Не удалось снять с публикации")
			return
		}
		writeJSON(w, 200, "Решение снято с публикации")
	case "favorite":
		profiFavoriteAction(w, r, id, u)
	case "purchase":
		profiPurchaseAction(w, r, id, u)
	default:
		writeJSON(w, 404, "Действие не найдено")
	}
}

func solutionToInput(x *profiSolution) profiSolutionInput {
	return profiSolutionInput{Type: x.Type, Title: x.Title, ShortDescription: x.ShortDescription, Description: x.Description, CoverImage: x.CoverImage, Price: x.Price, OldPrice: x.OldPrice, Currency: x.Currency, PricingType: x.PricingType, TrialDays: x.TrialDays, DeliveryType: x.DeliveryType, ExternalURL: x.ExternalURL, Tags: x.Tags, Topics: x.Topics, Audiences: x.Audiences, Sections: x.Sections, AccessFeatures: x.AccessFeatures, AIFeatures: x.AIFeatures, HowItWorks: x.HowItWorks, Media: x.Media, KeyMetrics: x.KeyMetrics, Bonuses: x.Bonuses, BonusStyle: x.BonusStyle, MetricStyle: x.MetricStyle, AccessStyle: x.AccessStyle, RightBlockTitle: x.RightBlockTitle, ImplementationTitle: x.ImplementationTitle, ImplementationSubtitle: x.ImplementationSubtitle, PurchaseButtonCode: x.PurchaseButtonCode, ProductData: x.ProductData}
}

func saveProfiSolution(ctx context.Context, id, userID int64, input profiSolutionInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE profimarket_solutions SET type=$1,title=$2,short_description=$3,description=$4,cover_image=$5,price=$6,old_price=$7,currency='RUB',pricing_type=$8,trial_days=$9,delivery_type=$10,external_url=$11,tags=ARRAY(SELECT jsonb_array_elements_text($12::jsonb)),topics=ARRAY(SELECT jsonb_array_elements_text($13::jsonb)),audiences=ARRAY(SELECT jsonb_array_elements_text($14::jsonb)),updated_at=NOW() WHERE id=$15 AND author_user_id=$16 AND deleted_at IS NULL`, input.Type, input.Title, input.ShortDescription, input.Description, input.CoverImage, input.Price, input.OldPrice, input.PricingType, input.TrialDays, input.DeliveryType, input.ExternalURL, stringArrayJSON(input.Tags), stringArrayJSON(input.Topics), stringArrayJSON(input.Audiences), id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("решение не найдено или недоступно")
	}
	metricsJSON, _ := json.Marshal(input.KeyMetrics)
	bonusesJSON, _ := json.Marshal(input.Bonuses)
	howItWorksJSON, _ := json.Marshal(input.HowItWorks)
	productDataJSON, _ := json.Marshal(input.ProductData)
	if _, err = tx.ExecContext(ctx, `UPDATE profimarket_solutions SET key_metrics=$1::jsonb,bonuses=$2::jsonb,bonus_style=$3,metric_style=$4,access_style=$5,right_block_title=$6,implementation_title=$7,implementation_subtitle=$8,purchase_button_code=CASE WHEN EXISTS(SELECT 1 FROM profimarket_purchase_button_options WHERE code=$9 AND active=TRUE) THEN $9 ELSE 'buy_and_implement' END,how_it_works=$10::jsonb,product_data=$11::jsonb WHERE id=$12 AND author_user_id=$13`, string(metricsJSON), string(bonusesJSON), input.BonusStyle, input.MetricStyle, input.AccessStyle, input.RightBlockTitle, input.ImplementationTitle, input.ImplementationSubtitle, input.PurchaseButtonCode, string(howItWorksJSON), string(productDataJSON), id, userID); err != nil {
		return err
	}
	for _, table := range []string{"profimarket_media", "profimarket_regulation_sections", "profimarket_access_features", "profimarket_ai_features", "profimarket_solution_crm", "profimarket_solution_platforms"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE solution_id=$1", id); err != nil {
			return err
		}
	}
	for i, m := range input.Media {
		m.Type = strings.ToUpper(m.Type)
		if m.Type != "IMAGE" && m.Type != "VIDEO" || strings.TrimSpace(m.URL) == "" {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO profimarket_media(solution_id,type,url,sort_order,is_preview) VALUES($1,$2,$3,$4,$5)`, id, m.Type, strings.TrimSpace(m.URL), i, m.IsPreview)
		if err != nil {
			return err
		}
	}
	if input.Type == "REGULATION" {
		for si, s := range input.Sections {
			if strings.TrimSpace(s.Title) == "" {
				continue
			}
			var sid int64
			numberingColor := strings.TrimSpace(s.NumberingColor)
			if len(numberingColor) != 7 || !strings.HasPrefix(numberingColor, "#") {
				numberingColor = ""
			}
			err = tx.QueryRowContext(ctx, `INSERT INTO profimarket_regulation_sections(solution_id,title,description,image_url,icon_image_url,numbering_color,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, id, strings.TrimSpace(s.Title), strings.TrimSpace(s.Description), strings.TrimSpace(s.ImageURL), strings.TrimSpace(s.IconImageURL), numberingColor, si).Scan(&sid)
			if err != nil {
				return err
			}
			for ii, item := range s.Items {
				if strings.TrimSpace(item.Title) == "" {
					continue
				}
				_, err = tx.ExecContext(ctx, `INSERT INTO profimarket_regulation_items(section_id,title,description,sort_order) VALUES($1,$2,$3,$4)`, sid, strings.TrimSpace(item.Title), strings.TrimSpace(item.Description), ii)
				if err != nil {
					return err
				}
			}
		}
		for i, f := range input.AccessFeatures {
			if strings.TrimSpace(f.Title) == "" {
				continue
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO profimarket_access_features(solution_id,icon,title,description,sort_order,text_color,background_color) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, strings.TrimSpace(f.Icon), strings.TrimSpace(f.Title), strings.TrimSpace(f.Description), i, strings.TrimSpace(f.TextColor), strings.TrimSpace(f.BackgroundColor))
			if err != nil {
				return err
			}
		}
		for _, cid := range input.CRMIDs {
			_, err = tx.ExecContext(ctx, `INSERT INTO profimarket_solution_crm(solution_id,crm_id) SELECT $1,id FROM profimarket_crm WHERE id=$2 AND active=TRUE ON CONFLICT DO NOTHING`, id, cid)
			if err != nil {
				return err
			}
		}
	} else {
		for i, f := range input.AIFeatures {
			if strings.TrimSpace(f.Title) == "" {
				continue
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO profimarket_ai_features(solution_id,icon,title,description,sort_order) VALUES($1,$2,$3,$4,$5)`, id, strings.TrimSpace(f.Icon), strings.TrimSpace(f.Title), strings.TrimSpace(f.Description), i)
			if err != nil {
				return err
			}
		}
		for _, pid := range input.PlatformIDs {
			_, err = tx.ExecContext(ctx, `INSERT INTO profimarket_solution_platforms(solution_id,platform_id) SELECT $1,id FROM profimarket_platforms WHERE id=$2 AND active=TRUE ON CONFLICT DO NOTHING`, id, pid)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func loadProfiSolution(ctx context.Context, key string, u *user) (*profiSolution, error) {
	x := &profiSolution{Sections: []profiSection{}, AccessFeatures: []profiFeature{}, AIFeatures: []profiFeature{}, Media: []profiMedia{}, CRMs: []profiDictionaryValue{}, Platforms: []profiDictionaryValue{}}
	var old sql.NullFloat64
	var pub sql.NullTime
	var tags, topics, audiences []byte
	query := `SELECT s.id,s.author_user_id,u.full_name,COALESCE(u.avatar_url,''),s.type,s.status,s.title,s.slug,s.short_description,s.description,s.cover_image,s.price,s.old_price,s.currency,s.pricing_type,s.trial_days,s.delivery_type,s.external_url,array_to_json(s.tags),array_to_json(s.topics),array_to_json(s.audiences),s.is_featured,s.is_new,s.views_count,s.published_at,s.created_at,s.updated_at,(SELECT COUNT(*) FROM profimarket_purchases p WHERE p.solution_id=s.id AND p.status='COMPLETED'),(SELECT COUNT(*) FROM profimarket_favorites f WHERE f.solution_id=s.id),COALESCE((SELECT AVG(r.rating) FROM profimarket_reviews r WHERE r.solution_id=s.id),0),(SELECT COUNT(*) FROM profimarket_reviews r WHERE r.solution_id=s.id) FROM profimarket_solutions s JOIN users u ON u.id=s.author_user_id WHERE s.deleted_at IS NULL AND (s.slug=$1 OR s.id::text=$1)`
	err := db.QueryRowContext(ctx, query, key).Scan(&x.ID, &x.AuthorUserID, &x.AuthorName, &x.AuthorAvatar, &x.Type, &x.Status, &x.Title, &x.Slug, &x.ShortDescription, &x.Description, &x.CoverImage, &x.Price, &old, &x.Currency, &x.PricingType, &x.TrialDays, &x.DeliveryType, &x.ExternalURL, &tags, &topics, &audiences, &x.IsFeatured, &x.IsNew, &x.ViewsCount, &pub, &x.CreatedAt, &x.UpdatedAt, &x.PurchasesCount, &x.FavoritesCount, &x.Rating, &x.ReviewCount)
	if err != nil {
		return nil, err
	}
	if old.Valid {
		x.OldPrice = &old.Float64
	}
	if pub.Valid {
		x.PublishedAt = &pub.Time
	}
	x.Tags = decodeStringArray(tags)
	x.Topics = decodeStringArray(topics)
	x.Audiences = decodeStringArray(audiences)
	if u != nil {
		x.IsAuthor = x.AuthorUserID == u.ID
		_ = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profimarket_favorites WHERE solution_id=$1 AND user_id=$2)`, x.ID, u.ID).Scan(&x.IsFavorite)
	}
	rows, _ := db.QueryContext(ctx, `SELECT id,type,url,sort_order,is_preview FROM profimarket_media WHERE solution_id=$1 ORDER BY sort_order,id`, x.ID)
	if rows != nil {
		for rows.Next() {
			var m profiMedia
			if rows.Scan(&m.ID, &m.Type, &m.URL, &m.SortOrder, &m.IsPreview) == nil {
				x.Media = append(x.Media, m)
			}
		}
		rows.Close()
	}
	rows, _ = db.QueryContext(ctx, `SELECT id,title,description,COALESCE(image_url,''),COALESCE(icon_image_url,''),COALESCE(numbering_color,''),sort_order FROM profimarket_regulation_sections WHERE solution_id=$1 ORDER BY sort_order,id`, x.ID)
	if rows != nil {
		for rows.Next() {
			var s profiSection
			if rows.Scan(&s.ID, &s.Title, &s.Description, &s.ImageURL, &s.IconImageURL, &s.NumberingColor, &s.SortOrder) != nil {
				continue
			}
			s.Items = []profiItem{}
			ir, _ := db.QueryContext(ctx, `SELECT id,title,description,sort_order FROM profimarket_regulation_items WHERE section_id=$1 ORDER BY sort_order,id`, s.ID)
			if ir != nil {
				for ir.Next() {
					var it profiItem
					if ir.Scan(&it.ID, &it.Title, &it.Description, &it.SortOrder) == nil {
						s.Items = append(s.Items, it)
					}
				}
				ir.Close()
			}
			x.Sections = append(x.Sections, s)
		}
		rows.Close()
	}
	loadFeatures := func(table string, target *[]profiFeature) {
		columns := "id,icon,title,description,sort_order"
		if table == "profimarket_access_features" {
			columns += ",text_color,background_color"
		}
		rs, _ := db.QueryContext(ctx, `SELECT `+columns+` FROM `+table+` WHERE solution_id=$1 ORDER BY sort_order,id`, x.ID)
		if rs != nil {
			for rs.Next() {
				var f profiFeature
				var scanErr error
				if table == "profimarket_access_features" {
					scanErr = rs.Scan(&f.ID, &f.Icon, &f.Title, &f.Description, &f.SortOrder, &f.TextColor, &f.BackgroundColor)
				} else {
					scanErr = rs.Scan(&f.ID, &f.Icon, &f.Title, &f.Description, &f.SortOrder)
				}
				if scanErr == nil {
					*target = append(*target, f)
				}
			}
			rs.Close()
		}
	}
	loadFeatures("profimarket_access_features", &x.AccessFeatures)
	loadFeatures("profimarket_ai_features", &x.AIFeatures)
	loadDict := func(query string, target *[]profiDictionaryValue) {
		rs, _ := db.QueryContext(ctx, query, x.ID)
		if rs != nil {
			for rs.Next() {
				var d profiDictionaryValue
				if rs.Scan(&d.ID, &d.Code, &d.Name, &d.Description, &d.Icon) == nil {
					*target = append(*target, d)
				}
			}
			rs.Close()
		}
	}
	loadDict(`SELECT c.id,c.code,c.name,c.description,c.icon FROM profimarket_solution_crm x JOIN profimarket_crm c ON c.id=x.crm_id WHERE x.solution_id=$1 ORDER BY c.sort_order,c.id`, &x.CRMs)
	loadDict(`SELECT p.id,p.code,p.name,'',p.icon FROM profimarket_solution_platforms x JOIN profimarket_platforms p ON p.id=x.platform_id WHERE x.solution_id=$1 ORDER BY p.sort_order,p.id`, &x.Platforms)
	var metricsJSON, bonusesJSON, howItWorksJSON, productDataJSON []byte
	if db.QueryRowContext(ctx, `SELECT s.key_metrics,s.bonuses,s.bonus_style,s.metric_style,s.access_style,s.right_block_title,s.implementation_title,s.implementation_subtitle,s.purchase_button_code,COALESCE(o.name,'Купить и внедрить'),s.how_it_works,s.product_data FROM profimarket_solutions s LEFT JOIN profimarket_purchase_button_options o ON o.code=s.purchase_button_code WHERE s.id=$1`, x.ID).Scan(&metricsJSON, &bonusesJSON, &x.BonusStyle, &x.MetricStyle, &x.AccessStyle, &x.RightBlockTitle, &x.ImplementationTitle, &x.ImplementationSubtitle, &x.PurchaseButtonCode, &x.PurchaseButtonLabel, &howItWorksJSON, &productDataJSON) == nil {
		_ = json.Unmarshal(metricsJSON, &x.KeyMetrics)
		_ = json.Unmarshal(bonusesJSON, &x.Bonuses)
		_ = json.Unmarshal(howItWorksJSON, &x.HowItWorks)
		_ = json.Unmarshal(productDataJSON, &x.ProductData)
	}
	return x, nil
}

func profiFavoriteAction(w http.ResponseWriter, r *http.Request, id int64, u *user) {
	if u == nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	switch r.Method {
	case http.MethodPost:
		_, err := db.ExecContext(r.Context(), `INSERT INTO profimarket_favorites(solution_id,user_id) SELECT id,$2 FROM profimarket_solutions WHERE id=$1 AND status='PUBLISHED' ON CONFLICT DO NOTHING`, id, u.ID)
		if err != nil {
			writeJSON(w, 500, "Не удалось добавить в избранное")
			return
		}
		profiRespond(w, 200, map[string]bool{"active": true})
	case http.MethodDelete:
		_, _ = db.ExecContext(r.Context(), `DELETE FROM profimarket_favorites WHERE solution_id=$1 AND user_id=$2`, id, u.ID)
		profiRespond(w, 200, map[string]bool{"active": false})
	default:
		writeJSON(w, 405, "Метод не поддерживается")
	}
}

type profiPurchaseInput struct {
	CRMID         *int64 `json:"crm_id"`
	CustomCRMName string `json:"custom_crm_name"`
	CRMEmail      string `json:"crm_email"`
	Comment       string `json:"comment"`
}

func profiPurchaseAction(w http.ResponseWriter, r *http.Request, id int64, u *user) {
	if r.Method != http.MethodPost || u == nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	var input profiPurchaseInput
	if !profiDecode(w, r, &input) {
		return
	}
	x, err := loadProfiSolution(r.Context(), strconv.FormatInt(id, 10), u)
	if err != nil || x.Status != "PUBLISHED" {
		writeJSON(w, 404, "Решение не найдено")
		return
	}
	if x.AuthorUserID == u.ID {
		writeJSON(w, 400, "Нельзя купить собственное решение")
		return
	}
	if x.Type == "REGULATION" {
		input.CRMEmail = strings.ToLower(strings.TrimSpace(input.CRMEmail))
		if _, e := mail.ParseAddress(input.CRMEmail); e != nil {
			writeJSON(w, 400, "Укажите e-mail учетной записи в CRM")
			return
		}
		if input.CRMID == nil {
			writeJSON(w, 400, "Выберите CRM")
			return
		}
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось оформить покупку")
		return
	}
	defer tx.Rollback()
	var purchaseID int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO profimarket_purchases(solution_id,buyer_user_id,seller_user_id,amount,currency,pricing_type,status,product_title_snapshot,product_slug_snapshot,product_type_snapshot,product_cover_snapshot,product_description_snapshot,seller_name_snapshot) VALUES($1,$2,$3,$4,$5,$6,'COMPLETED',$7,$8,$9,$10,$11,$12) RETURNING id`, x.ID, u.ID, x.AuthorUserID, x.Price, x.Currency, x.PricingType, x.Title, x.Slug, x.Type, x.CoverImage, x.ShortDescription, x.AuthorName).Scan(&purchaseID)
	if err != nil {
		writeJSON(w, 500, "Не удалось оформить покупку")
		return
	}
	if x.Type == "REGULATION" {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO profimarket_implementation_requests(purchase_id,solution_id,buyer_user_id,seller_user_id,crm_id,custom_crm_name,crm_email,comment) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, purchaseID, x.ID, u.ID, x.AuthorUserID, input.CRMID, strings.TrimSpace(input.CustomCRMName), input.CRMEmail, strings.TrimSpace(input.Comment))
		if err != nil {
			writeJSON(w, 400, "Не удалось создать запрос на внедрение")
			return
		}
	}
	actionTitle := "Новая покупка"
	if x.Type == "AI_ASSISTANT" && x.TrialDays > 0 {
		actionTitle = "Новая заявка на бесплатный период"
	}
	notificationBody := fmt.Sprintf("%s (%s) заинтересовался продуктом «%s». Свяжитесь с покупателем по e-mail.", u.FullName, u.Email, x.Title)
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notifications(user_id,type,title,body,entity_type,entity_id) VALUES($1,'profimarket_order',$2,$3,'profimarket_purchase',$4)`, x.AuthorUserID, actionTitle, notificationBody, purchaseID); err != nil {
		writeJSON(w, 500, "Не удалось уведомить продавца о заказе")
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, "Не удалось завершить покупку")
		return
	}
	var sellerName, sellerEmail string
	if db.QueryRowContext(r.Context(), `SELECT full_name,email FROM users WHERE id=$1`, x.AuthorUserID).Scan(&sellerName, &sellerEmail) == nil {
		priceText := fmt.Sprintf("%.0f ₽", x.Price)
		if x.PricingType == "FREE" || x.Price == 0 {
			priceText = "Бесплатно"
		}
		emailData := profiMarketOrderEmailData{BuyerName: u.FullName, BuyerEmail: u.Email, ProductTitle: x.Title, ActionTitle: actionTitle, PriceText: priceText, PurchaseID: purchaseID}
		var emailErr error
		for attempt := 1; attempt <= 2; attempt++ {
			emailErr = sendProfiMarketOrderEmail(sellerName, sellerEmail, emailData)
			if emailErr == nil {
				log.Printf("profimarket order email sent to seller %d for purchase %d", x.AuthorUserID, purchaseID)
				break
			}
			log.Printf("profimarket order email attempt %d to seller %d for purchase %d: %v", attempt, x.AuthorUserID, purchaseID, emailErr)
			if attempt == 1 {
				time.Sleep(400 * time.Millisecond)
			}
		}
	}
	message := "Покупка оформлена. Автор получил ваши контакты и свяжется с вами."
	if x.Type == "AI_ASSISTANT" && x.TrialDays > 0 {
		message = "Заявка на бесплатный период отправлена автору. Он получил ваши контакты и свяжется с вами."
	}
	profiRespond(w, 201, map[string]any{"purchase_id": purchaseID, "message": message})
}

func profiMarketMetaAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	if err := syncProfiMarketCRMs(r.Context()); err != nil {
		writeJSON(w, 500, "Не удалось загрузить справочник CRM")
		return
	}
	crms, platforms, purchaseButtons, onecConfigurations, compatibility := []profiDictionaryValue{}, []profiDictionaryValue{}, []profiDictionaryValue{}, []profiDictionaryValue{}, []profiDictionaryValue{}
	load := func(table string, target *[]profiDictionaryValue) {
		columns := "id,code,name,'',''"
		if table == "profimarket_crm" {
			columns = "id,code,name,description,icon"
		} else if table == "profimarket_platforms" {
			columns = "id,code,name,'',icon"
		}
		rows, _ := db.QueryContext(r.Context(), `SELECT `+columns+` FROM `+table+` WHERE active=TRUE ORDER BY sort_order,id`)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var d profiDictionaryValue
				if rows.Scan(&d.ID, &d.Code, &d.Name, &d.Description, &d.Icon) == nil {
					*target = append(*target, d)
				}
			}
		}
	}
	load("profimarket_crm", &crms)
	load("profimarket_platforms", &platforms)
	load("profimarket_compatibility_options", &compatibility)
	rows, _ := db.QueryContext(r.Context(), `SELECT id,code,name,'',logo FROM profimarket_onec_configurations WHERE active=TRUE ORDER BY sort_order,id`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var option profiDictionaryValue
			if rows.Scan(&option.ID, &option.Code, &option.Name, &option.Description, &option.Icon) == nil {
				onecConfigurations = append(onecConfigurations, option)
			}
		}
	}
	rows, _ = db.QueryContext(r.Context(), `SELECT id,code,name,'','' FROM profimarket_purchase_button_options WHERE active=TRUE ORDER BY sort_order,id`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var option profiDictionaryValue
			if rows.Scan(&option.ID, &option.Code, &option.Name, &option.Description, &option.Icon) == nil {
				purchaseButtons = append(purchaseButtons, option)
			}
		}
	}
	categoryNames := map[string]string{"AI_ASSISTANT": "ИИ-ассистенты", "REGULATION": "Регламенты", "AUTOMATION": "Автоматизации", "INSTRUCTION": "Инструкции", "ONEC_INTEGRATION": "1С Интеграции", "TEMPLATE": "Шаблоны", "CHECKLIST": "Чек-листы"}
	categoryOrder := []string{"AI_ASSISTANT", "REGULATION", "AUTOMATION", "INSTRUCTION", "ONEC_INTEGRATION", "TEMPLATE", "CHECKLIST"}
	counts := map[string]int{}
	categoryRows, _ := db.QueryContext(r.Context(), `SELECT type,COUNT(*) FROM profimarket_solutions WHERE status='PUBLISHED' AND deleted_at IS NULL GROUP BY type`)
	if categoryRows != nil {
		defer categoryRows.Close()
		for categoryRows.Next() {
			var productType string
			var count int
			if categoryRows.Scan(&productType, &count) == nil {
				counts[productType] = count
			}
		}
	}
	categories := make([]map[string]any, 0, len(categoryOrder))
	for _, productType := range categoryOrder {
		categories = append(categories, map[string]any{"type": productType, "name": categoryNames[productType], "count": counts[productType]})
	}
	profiRespond(w, 200, map[string]any{"crms": crms, "platforms": platforms, "onec_configurations": onecConfigurations, "compatibility": compatibility, "purchase_buttons": purchaseButtons, "categories": categories})
}

func profiMarketMySolutionsAPI(w http.ResponseWriter, r *http.Request) {
	u, ok := profiRequireUser(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	profiMarketList(w, r, true, u.ID)
}
func profiMarketMyPurchasesAPI(w http.ResponseWriter, r *http.Request) {
	u, ok := profiRequireUser(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	profiPurchaseList(w, r, `p.buyer_user_id=$1`, u.ID)
}
func profiMarketMyOrdersAPI(w http.ResponseWriter, r *http.Request) {
	u, ok := profiRequireUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		var input struct {
			OrderID int64 `json:"order_id"`
		}
		if !profiDecode(w, r, &input) {
			return
		}
		if input.OrderID <= 0 {
			writeJSON(w, 400, "Некорректный заказ")
			return
		}
		result, err := db.ExecContext(r.Context(), `UPDATE profimarket_purchases SET seller_seen_at=NOW() WHERE id=$1 AND seller_user_id=$2 AND seller_seen_at IS NULL`, input.OrderID, u.ID)
		if err != nil {
			writeJSON(w, 500, "Не удалось обновить уведомления")
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			writeJSON(w, 404, "Новый заказ не найден")
			return
		}
		profiRespond(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	if r.URL.Query().Get("summary") == "1" {
		var total, unread int
		if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*),COUNT(*) FILTER(WHERE seller_seen_at IS NULL) FROM profimarket_purchases WHERE seller_user_id=$1`, u.ID).Scan(&total, &unread); err != nil {
			writeJSON(w, 500, "Не удалось загрузить уведомления")
			return
		}
		profiRespond(w, 200, map[string]int{"total_count": total, "new_count": unread})
		return
	}
	profiPurchaseList(w, r, `p.seller_user_id=$1`, u.ID)
}
func profiPurchaseList(w http.ResponseWriter, r *http.Request, where string, userID int64) {
	rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.solution_id,
		COALESCE(NULLIF(p.product_title_snapshot,''),s.title,'Удалённое решение'),
		COALESCE(NULLIF(p.product_slug_snapshot,''),s.slug,''),
		COALESCE(NULLIF(p.product_type_snapshot,''),s.type,''),
		COALESCE(NULLIF(p.product_cover_snapshot,''),s.cover_image,''),
		COALESCE(NULLIF(p.product_description_snapshot,''),s.short_description,''),
		COALESCE(NULLIF(p.seller_name_snapshot,''),seller.full_name,'Автор FinTalent'),
		p.amount,p.currency,p.pricing_type,p.status,p.created_at,b.full_name,b.email,
		COALESCE(c.name,''),COALESCE(ir.custom_crm_name,''),COALESCE(ir.crm_email,''),COALESCE(ir.comment,''),COALESCE(ir.status,''),
		p.seller_seen_at IS NULL,(s.id IS NOT NULL AND s.deleted_at IS NULL)
		FROM profimarket_purchases p
		LEFT JOIN profimarket_solutions s ON s.id=p.solution_id
		LEFT JOIN users seller ON seller.id=p.seller_user_id
		JOIN users b ON b.id=p.buyer_user_id
		LEFT JOIN profimarket_implementation_requests ir ON ir.purchase_id=p.id
		LEFT JOIN profimarket_crm c ON c.id=ir.crm_id
		WHERE `+where+` ORDER BY p.created_at DESC`, userID)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить покупки")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	newCount := 0
	for rows.Next() {
		var id, sid int64
		var title, slug, typ, cover, description, author, currency, pricing, status, buyer, buyerEmail, crm, customCRM, crmEmail, comment, implementation string
		var amount float64
		var isNew, solutionAvailable bool
		var created time.Time
		if rows.Scan(&id, &sid, &title, &slug, &typ, &cover, &description, &author, &amount, &currency, &pricing, &status, &created, &buyer, &buyerEmail, &crm, &customCRM, &crmEmail, &comment, &implementation, &isNew, &solutionAvailable) == nil {
			if isNew {
				newCount++
			}
			items = append(items, map[string]any{"id": id, "solution_id": sid, "title": title, "slug": slug, "type": typ, "cover_image": cover, "short_description": description, "author_name": author, "amount": amount, "currency": currency, "pricing_type": pricing, "status": status, "created_at": created, "buyer_name": buyer, "buyer_email": buyerEmail, "crm": crm, "custom_crm_name": customCRM, "crm_email": crmEmail, "comment": comment, "implementation_status": implementation, "is_new": isNew, "solution_available": solutionAvailable})
		}
	}
	profiRespond(w, 200, map[string]any{"items": items, "total_count": len(items), "new_count": newCount})
}

func profiMarketUploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	if _, ok := profiRequireUser(w, r); !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	if err := r.ParseMultipartForm(7 << 20); err != nil {
		writeJSON(w, 400, "Файл слишком большой")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, 400, "Выберите изображение")
		return
	}
	defer file.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	mime := http.DetectContentType(head[:n])
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}[mime]
	if ext == "" {
		writeJSON(w, 400, "Поддерживаются JPG, PNG, WebP и GIF")
		return
	}
	token := make([]byte, 16)
	_, _ = rand.Read(token)
	dir := filepath.Join("static", "uploads", "profimarket")
	if err = os.MkdirAll(dir, 0755); err != nil {
		writeJSON(w, 500, "Не удалось сохранить файл")
		return
	}
	name := hex.EncodeToString(token) + ext
	out, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить файл")
		return
	}
	defer out.Close()
	if _, err = out.Write(head[:n]); err == nil {
		_, err = io.Copy(out, io.LimitReader(file, 7<<20))
	}
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить файл")
		return
	}
	profiRespond(w, 201, map[string]string{"url": "/static/uploads/profimarket/" + name})
}
