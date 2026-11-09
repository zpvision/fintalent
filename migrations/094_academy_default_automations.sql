-- Create independent, editable scenarios for each course. Never recreate a
-- removed/paused default or replace a scenario already configured by its author.
CREATE OR REPLACE FUNCTION academy_seed_course_automations(cid bigint, uid bigint) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO academy_automations(course_id,created_by,name,trigger,config,status,active_since,request_key)
 SELECT cid,uid,t.name,t.event,jsonb_build_object(
  'audience',jsonb_build_object('join','and','conditions','[]'::jsonb),
  'inactive_days',7,'lead_days',1,
  'steps',jsonb_build_array(jsonb_build_object('key','default-email','channel','email','category','service','target','student','timing','immediate','delay_minutes',0,
   'message',jsonb_build_object('subject',t.subject,'preheader',t.name,'blocks',jsonb_build_array(
    jsonb_build_object('key','default-heading','type','heading','text',t.name,'align','left'),
    jsonb_build_object('key','default-body','type','text','text',t.body,'align','left'),
    jsonb_build_object('key','course-link','type','button','text','Открыть обучение','url','{{lesson.url}}','color','#3569e8','align','left')))))),
  'draft',NULL,'academy-default-'||cid||'-'||t.event
 FROM (VALUES
  ('enrolled','Ученик зачислен — письмо ученику','Добро пожаловать на курс!','Вы зачислены на курс. Откройте учебную комнату, чтобы познакомиться с программой и начать обучение.'),
  ('course_completed','Курс завершён — письмо ученику','Поздравляем с завершением курса!','Вы завершили обучение. Поздравляем с достижением! Материалы курса доступны в вашей учебной комнате в течение срока доступа.')
 ) t(event,name,subject,body)
 WHERE NOT EXISTS(SELECT 1 FROM academy_automations a WHERE a.course_id=cid AND a.trigger=t.event)
 ON CONFLICT(request_key) DO NOTHING;
END $$;
CREATE OR REPLACE FUNCTION academy_seed_new_course_automations() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM academy_seed_course_automations(NEW.id,NEW.owner_id);
 RETURN NEW;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_default_course_automations' AND tgrelid='courses'::regclass) THEN
  CREATE TRIGGER academy_default_course_automations AFTER INSERT ON courses FOR EACH ROW EXECUTE FUNCTION academy_seed_new_course_automations();
 END IF;
END $$;
SELECT academy_seed_course_automations(id,owner_id) FROM courses;
