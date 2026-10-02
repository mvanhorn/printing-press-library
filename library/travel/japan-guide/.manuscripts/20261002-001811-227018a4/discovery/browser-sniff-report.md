# Browser discovery proof
Primary goal: find source destinations, attractions and visit facts. Native CUA Chrome isolated task tab, no auth.

1. Opened https://www.japan-guide.com/e/e623a.html (visible destination directory, source editorial recommendation).
2. Clicked Tokyo link, reached https://www.japan-guide.com/e/e2164.html (destination, district-grouped attractions, source interest tags, side trips, itineraries).
3. Clicked Sensoji Temple, reached https://www.japan-guide.com/e/e3001.html (attraction, Japanese name 浅草寺, source hours/closure/admission, source update May 21 2025).

DOM inspection found .dot_rating__dots data-dots and data-tooltip-label; separate visitor rating .place_details__number. Visit facts in .page_admission, .page_admission__item_label, .page_admission__item_content; article prose excluded. Native CDP response event capability returned no events (tool capability limitation); public document request shape obtained from navigation URLs and independent HTTP response headers. No credentials, cookies, API keys or authenticated requests captured. No CAPTCHA encountered.

GET public pages each returned 200 text/html;charset=shift-jis outside sandbox. /e/e623a.html, /e/e2164.html, /e/e3001.html, /e/e623.html, /e/e2400.html. No JSON API required. Structured static HTML matches rendered facts; standard HTTP runtime verified.
