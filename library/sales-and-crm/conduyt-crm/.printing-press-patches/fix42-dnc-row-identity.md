# Fix 42: suppression-row identity

Require every DNC and contact-correlation row to carry a stable non-blank contact ID or normalizable phone. Malformed rows now produce partial API-class results instead of being silently ignored, with regressions for missing/blank identities and invalid DNC values.
