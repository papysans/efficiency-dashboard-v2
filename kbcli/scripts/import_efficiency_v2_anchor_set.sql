\set ON_ERROR_STOP on

CREATE TEMP TABLE tmp_efficiency_v2_anchor_set (
  anchor_id text,
  source text,
  source_version text,
  anchor_kind text,
  human_labeled_minutes double precision,
  without_ai_minutes double precision,
  human_labeled boolean,
  weight double precision,
  feature_vector jsonb,
  labels jsonb,
  valid_from timestamptz,
  valid_to timestamptz
);

\copy tmp_efficiency_v2_anchor_set FROM 'docs/data/efficiency_v2_anchor_set.csv' WITH (FORMAT csv, HEADER true)

INSERT INTO anchor_set (
  anchor_id,
  source,
  source_version,
  anchor_kind,
  human_labeled_minutes,
  without_ai_minutes,
  human_labeled,
  weight,
  feature_vector,
  labels,
  valid_from,
  valid_to,
  updated_at
)
SELECT
  anchor_id,
  source,
  COALESCE(source_version, ''),
  anchor_kind,
  human_labeled_minutes,
  without_ai_minutes,
  COALESCE(human_labeled, false),
  COALESCE(NULLIF(weight, 0), 1),
  COALESCE(feature_vector, '{}'::jsonb),
  COALESCE(labels, '{}'::jsonb),
  valid_from,
  valid_to,
  now()
FROM tmp_efficiency_v2_anchor_set
WHERE anchor_id IS NOT NULL
  AND anchor_id <> ''
  AND source IS NOT NULL
  AND source <> ''
  AND without_ai_minutes IS NOT NULL
  AND feature_vector IS NOT NULL
  AND feature_vector <> '{}'::jsonb
ON CONFLICT (anchor_id) DO UPDATE SET
  source = EXCLUDED.source,
  source_version = EXCLUDED.source_version,
  anchor_kind = EXCLUDED.anchor_kind,
  human_labeled_minutes = EXCLUDED.human_labeled_minutes,
  without_ai_minutes = EXCLUDED.without_ai_minutes,
  human_labeled = EXCLUDED.human_labeled,
  weight = EXCLUDED.weight,
  feature_vector = EXCLUDED.feature_vector,
  labels = EXCLUDED.labels,
  valid_from = EXCLUDED.valid_from,
  valid_to = EXCLUDED.valid_to,
  updated_at = now();

DROP TABLE tmp_efficiency_v2_anchor_set;
