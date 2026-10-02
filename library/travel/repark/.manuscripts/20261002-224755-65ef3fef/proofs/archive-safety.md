# Archive protection

No user credential or authenticated browser session was used. Public source HAR request Authorization/Cookie/API-key headers, response Set-Cookie headers, cookie arrays and auth-like query values were stripped; HAR response bodies were removed. Browser-served map keys and raw Set-Cookie header lines were redacted before embedding or archiving. Public parking facts, source field names and verified request contracts remain. Strict PII audit of the shipping tree plus research was clean; archive scanning is an additional gate. No live session state is retained in discovery or embedded manuscripts.

The whole-archive scan found two URL-encoding false positives for Repark’s public Tokyo Station postal label (encoded 〒 plus 100-0005 looked like a US ZIP). Both were accepted individually as api_provider_data with quoted source URL context; final strict scan has no pending findings.
