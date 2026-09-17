# Fix 38: scorecard target periods

Select only the target period matching an exact day, week, month, or quarter
window, or require `--period` for ambiguous windows. Scorecard rows now identify
their period, and JSON and human output report skipped target counts and periods.
Regression tests cover inference, ambiguity, explicit selection, and mixed periods.
