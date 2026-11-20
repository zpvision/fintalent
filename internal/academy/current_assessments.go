package academy

// currentAssessmentCTEs resolves active blocks once per program. Callers provide
// current_programs (id, content, publication_id) and selected_assessments scoped
// to those programs. Preserve the first-publication fallback for legacy rows.
const currentAssessmentCTEs = `published_blocks AS MATERIALIZED (
 SELECT v.id version_id,v.publication_id,l->>'key' lesson_key,b->>'key' block_key,b->>'type' kind
 FROM current_programs v CROSS JOIN LATERAL jsonb_array_elements(v.content->'modules') m
 CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l CROSS JOIN LATERAL jsonb_array_elements(l->'blocks') b
 WHERE m->>'status'='published' AND l->>'status'='published' AND b->>'type' IN('test','assignment')
), active_assessments AS MATERIALIZED (
 SELECT a.id FROM selected_assessments a JOIN published_blocks b
 ON b.version_id=a.program_version_id AND b.lesson_key=a.lesson_key AND b.block_key=a.block_key AND b.kind=a.kind
 WHERE a.publication_revision_id=b.publication_id OR a.publication_revision_id IS NULL
 AND (b.publication_id=0 OR EXISTS(SELECT 1 FROM academy_program_revisions r WHERE r.id=b.publication_id AND r.version_id=b.version_id AND r.number=1))
 AND NOT EXISTS(SELECT 1 FROM selected_assessments fresh WHERE fresh.program_version_id=a.program_version_id AND fresh.lesson_key=a.lesson_key AND fresh.block_key=a.block_key AND fresh.publication_revision_id=b.publication_id)
)`
