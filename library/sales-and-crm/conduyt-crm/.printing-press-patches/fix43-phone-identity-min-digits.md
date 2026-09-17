# Fix 43: canonical phone identities

Use one phone-identity normalizer for send-check suppression matching, import
correlation, and line-type estimates. It strips formatting, equates a leading
US/Canada country code with bare ten-digit values, and rejects shorter values;
unmappable safety inputs produce partial API-class results rather than `go`.
