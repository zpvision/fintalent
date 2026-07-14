--
-- PostgreSQL database dump
--

\restrict tqYVGIjkS9Wn2E3YaMbyklWjya0dnvfZzUCk0qKSoYGRbmIDEZrMGQseYAzdvCi

-- Dumped from database version 17.10
-- Dumped by pg_dump version 17.10

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Data for Name: dictionaries; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (2, 'Опыт', '2026-07-14 20:39:06.857339+00', '2026-07-14 20:39:06.857339+00', 'experience');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (3, 'Сфера деятельности', '2026-07-14 20:39:06.869583+00', '2026-07-14 20:39:06.869583+00', 'business_sector');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (4, 'Размер компании', '2026-07-14 20:39:06.895583+00', '2026-07-14 20:39:06.895583+00', 'company_size');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (14, 'Участки', '2026-07-14 21:01:52.324971+00', '2026-07-14 21:01:52.324971+00', 'accounting_areas');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (15, 'Программы', '2026-07-14 21:01:52.350362+00', '2026-07-14 21:01:52.350362+00', 'software');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (16, 'Сколько компаний вели одновременно?', '2026-07-14 21:01:52.36894+00', '2026-07-14 21:01:52.36894+00', 'companies_managed_simultaneously');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (17, 'Сколько юридических лиц вели в общей сложности?', '2026-07-14 21:01:52.383013+00', '2026-07-14 21:01:52.383013+00', 'legal_entities_managed_total');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (18, 'Объем первичных документов в месяц (примерно)?', '2026-07-14 21:01:52.394088+00', '2026-07-14 21:01:52.394088+00', 'monthly_primary_documents');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (19, 'Сколько сотрудников было в расчете?', '2026-07-14 21:01:52.405392+00', '2026-07-14 21:01:52.405392+00', 'employees_in_payroll');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (20, 'С максимальным оборотом каких компаний работали?', '2026-07-14 21:01:52.416773+00', '2026-07-14 21:01:52.416773+00', 'maximum_company_turnover');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (21, 'Проходили налоговые проверки?', '2026-07-14 21:01:52.426819+00', '2026-07-14 21:01:52.426819+00', 'tax_audits');
INSERT INTO public.dictionaries (id, name, created_at, updated_at, alias) VALUES (1, 'Кем вы хотите работать?', '2026-07-14 20:39:06.827378+00', '2026-07-14 21:06:40.492488+00', 'position');


--
-- Data for Name: dictionary_items; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (14, 2, 'Нет опыта', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (15, 2, 'До 1 года', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (16, 2, '1–3 года', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (17, 2, '3–5 лет', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (18, 2, '5–10 лет', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (19, 2, 'Более 10 лет', 5, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (20, 3, 'Производство', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (21, 3, 'Торговля', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (22, 3, 'Услуги', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (23, 3, 'Строительство', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (24, 3, 'IT', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (25, 3, 'Маркетплейсы', 5, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (26, 3, 'Общепит', 6, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (27, 3, 'Медицина', 7, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (28, 3, 'Образование', 8, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (29, 3, 'Государственные учреждения', 9, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (30, 3, 'Некоммерческие организации', 10, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (31, 3, 'Логистика', 11, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (32, 3, 'Другое', 12, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (33, 4, 'До 10 сотрудников', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (34, 4, 'До 30', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (35, 4, 'До 100', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (36, 4, 'До 300', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (37, 4, 'Более 300', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (64, 15, '1С:Бухгалтерия', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (65, 15, '1С:ЗУП', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (66, 15, '1С:ERP', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (67, 15, 'СБИС', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (68, 15, 'Контур.Экстерн', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (69, 15, 'Диадок', 5, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (70, 15, 'Excel', 6, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (71, 15, 'Мое дело', 7, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (72, 16, '1', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (73, 16, '2-5', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (74, 16, '6-10', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (75, 16, '11-20', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (76, 16, '20-50', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (77, 16, 'Более 50', 5, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (78, 17, '1-5', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (79, 17, '6-20', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (80, 17, '21-50', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (81, 17, '51-100', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (82, 17, 'Более 100', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (83, 18, 'До 100', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (84, 18, '100-500', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (85, 18, '500-1000', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (86, 18, '1000-5000', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (87, 18, 'Более 5000', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (88, 19, 'До 10', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (89, 19, '10-50', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (90, 19, '51-100', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (91, 19, '101-200', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (92, 19, 'Более 200', 4, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (93, 20, 'До 30 млн ₽', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (94, 20, '30-100 млн ₽', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (95, 20, '100-500 млн ₽', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (96, 20, 'Более 500 млн ₽', 3, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (97, 21, 'Нет', 0, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (98, 21, 'Да, 1–2 раза', 1, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (99, 21, 'Да, регулярно', 2, '', '');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (101, 1, 'Главный бухгалтер', 0, 'Руководство бухгалтерией и отчетностью', '/api/assets/position-icon/0.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (102, 1, 'Заместитель главного бухгалтера', 1, 'Поддержка главного бухгалтера и контроль учета', '/api/assets/position-icon/1.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (103, 1, 'Бухгалтер', 2, 'Ведение бухгалтерского и налогового учета', '/api/assets/position-icon/2.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (104, 1, 'Помощник бухгалтера', 3, 'Поддержка бухгалтерии и выполнение поручений', '/api/assets/position-icon/3.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (105, 1, 'Бухгалтер по заработной плате', 4, 'Расчет заработной платы и кадровый учет', '/api/assets/position-icon/4.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (106, 1, 'Бухгалтер по первичной документации', 5, 'Первичные документы и их обработка', '/api/assets/position-icon/5.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (54, 14, 'НДС', 0, 'Налог на добавленную стоимость', '/api/assets/accounting-area-icon/0.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (107, 1, 'Бухгалтер по налогам', 6, 'Расчет налогов и подготовка деклараций', '/api/assets/position-icon/6.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (108, 1, 'Финансовый бухгалтер', 7, 'Финансовый учет и управленческая отчетность', '/api/assets/position-icon/7.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (109, 1, 'Аудитор', 8, 'Проверка учета и финансовой отчетности', '/api/assets/position-icon/8.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (110, 1, 'Налоговый консультант', 9, 'Консультации по налогам и законодательству', '/api/assets/position-icon/9.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (111, 1, 'Финансовый аналитик', 10, 'Анализ финансовых данных и планирование', '/api/assets/position-icon/10.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (112, 1, 'Экономист', 11, 'Планирование, бюджеты и экономический анализ', '/api/assets/position-icon/11.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (113, 1, 'Другой вариант', 12, 'Укажите подходящую должность самостоятельно', '/api/assets/position-icon/12.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (114, 1, 'Еще вариант', 13, 'Дополнительный вариант должности', '/api/assets/position-icon/13.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (55, 14, 'УСН', 1, 'Упрощенная система налогообложения', '/api/assets/accounting-area-icon/1.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (56, 14, 'ОСНО', 2, 'Общая система налогообложения', '/api/assets/accounting-area-icon/2.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (57, 14, 'Зарплата и кадры', 3, 'Расчет зарплаты и кадровый учет', '/api/assets/accounting-area-icon/3.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (58, 14, 'ТМЦ', 4, 'Товары, материалы и складской учет', '/api/assets/accounting-area-icon/4.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (59, 14, 'Банк и касса', 5, 'Банковские операции и кассовые документы', '/api/assets/accounting-area-icon/5.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (60, 14, 'Основные средства', 6, 'Учет основных средств и амортизация', '/api/assets/accounting-area-icon/6.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (61, 14, 'Отчетность', 7, 'Подготовка и сдача отчетности', '/api/assets/accounting-area-icon/7.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (62, 14, 'ВЭД', 8, 'Внешнеэкономическая деятельность', '/api/assets/accounting-area-icon/8.png');
INSERT INTO public.dictionary_items (id, dictionary_id, value, sort_order, comment, icon) VALUES (63, 14, 'Производство', 9, 'Производственный учет и калькуляция', '/api/assets/accounting-area-icon/9.png');


--
-- Name: dictionaries_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.dictionaries_id_seq', 24, true);


--
-- Name: dictionary_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.dictionary_items_id_seq', 114, true);


--
-- PostgreSQL database dump complete
--

\unrestrict tqYVGIjkS9Wn2E3YaMbyklWjya0dnvfZzUCk0qKSoYGRbmIDEZrMGQseYAzdvCi

