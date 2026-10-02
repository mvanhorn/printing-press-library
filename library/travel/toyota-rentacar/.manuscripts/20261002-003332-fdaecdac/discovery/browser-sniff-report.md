# Toyota browser discovery
Primary goal: resolve shops, inspect a dated class estimate, and hand off before personal information or booking.

Native Chrome extension CUA tab448471359 resumed from the prior discovery. The real Tokyo Nihonbashi Oct20–21 09:00 C1 customer-information page showed11,990JPY total/tax1,090JPY. Changing ETC required did not recalculate the amount; stop before customer details/terms. Source initial amount is not a complete inclusive quote.

Keyword search through rendered pickup modal produced /eng/reservation/index01.aspx?keyword=Kyoto%20Station&shopMode=0, heading Around Kyoto Station,10 shops, Kyoto Shinkansen/京都駅新幹線口店 first. Public first-party GET reproduced Kyoto and Narita pages with distinct shop identities.

Anonymous in-memory cookiejar replay: GET selected pickup -> populated successful form fields and dates -> POST lnkBtnSearch -> recommended -> POST lnkChangeCarPc -> index02. Date context echoes and real prices. Class selection POST returns first-party network-error page despite source-populated shop/date fields; unsupported for final quote. Customer details never submitted.

One-way native tab448471415: public simulation -> Search store -> Tokyo Nihonbashi63601:01V -> Hatchobori63601:095 return -> Confirm -> Standard family -> calculate. Source displayed0JPY including10%tax. Anonymous HTTP sequence reproduced the same result with no authentication. Fee simulation is independent of date inventory and does not guarantee the route/class can be booked.

No CAPTCHA,login,challenge or source access restriction observed. Stdlib HTTP works outside restricted-tool DNS environment. No browser required at runtime. Browser DOM/visible UI and structured HTTP summaries are evidence; CDP event collection returned no events and is not represented as captured HAR proof. No cookies,personal fields,VIEWSTATE or EVENTVALIDATION are preserved.
