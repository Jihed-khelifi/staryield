-- Growth: the tarot birth card calculator is counted like the other calculators.
ALTER TABLE calculator_visits DROP CONSTRAINT calculator_visits_kind_check;
ALTER TABLE calculator_visits
    ADD CONSTRAINT calculator_visits_kind_check CHECK (kind IN ('natal-chart', 'path-of-life', 'astrology', 'tarot-birth-card'));
