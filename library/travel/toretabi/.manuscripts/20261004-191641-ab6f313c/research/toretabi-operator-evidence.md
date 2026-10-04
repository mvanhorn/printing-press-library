# Linked official operator evidence

Ordinary anonymous CLI GETs to Toretabi supplied these official links. Statuses concern the bounded linked document read, not ticket inventory. No credentials, browser or bypass was used.

[
  {
    "id": "tokai_043",
    "source_url": "https://www.toretabi.jp/ticket/tokai_043.html",
    "operator_urls": [
      "https://railway.jr-central.co.jp/tickets/noritetsu-tabikippu-17/"
    ],
    "publisher_observed_at": "2026-10-04T12:50:57.932465Z",
    "operator_status": "partial_rule_evidence",
    "operator_observed_at": "2026-10-04T12:50:58.259181Z"
  },
  {
    "id": "hokkaido_028",
    "source_url": "https://www.toretabi.jp/ticket/hokkaido_028.html",
    "operator_urls": [
      "https://www.jrhokkaido.co.jp/CM/Otoku/007246/"
    ],
    "publisher_observed_at": "2026-10-04T12:50:59.508371Z",
    "operator_status": "partial_rule_evidence",
    "operator_observed_at": "2026-10-04T12:51:00.728862Z"
  },
  {
    "id": "east_027",
    "source_url": "https://www.toretabi.jp/ticket/east_027.html",
    "operator_urls": [
      "https://www.jreast.co.jp/tickets/info.aspx?GoodsCd=2993"
    ],
    "publisher_observed_at": "2026-10-04T12:51:01.373873Z",
    "operator_status": "identity_unverified",
    "operator_observed_at": "2026-10-04T12:51:01.654151Z"
  },
  {
    "id": "tokai_043",
    "name_ja": "JR東海＆17私鉄 乗り鉄☆たびきっぷ",
    "source_url": "https://www.toretabi.jp/ticket/tokai_043.html",
    "observed_at": "2026-10-04T12:58:10.420927Z",
    "operator_urls": [
      "https://railway.jr-central.co.jp/tickets/noritetsu-tabikippu-17/"
    ],
    "operator_status": "partial_rule_evidence",
    "operator_http_status": 200,
    "operator_observed_at": "2026-10-04T12:58:10.623549Z"
  },
  {
    "id": "shikoku_030",
    "name_ja": "6枚回数券",
    "source_url": "https://www.toretabi.jp/ticket/shikoku_030.html",
    "observed_at": "2026-10-04T12:58:13.850997Z",
    "operator_urls": [
      "https://www.jr-eki.com/ticket/brand/2-2BW"
    ],
    "operator_status": "unsupported_operator",
    "operator_http_status": null,
    "operator_observed_at": null
  },
  {
    "id": "kyushu_013",
    "name_ja": "2枚きっぷ（乗車券）",
    "source_url": "https://www.toretabi.jp/ticket/kyushu_013.html",
    "observed_at": "2026-10-04T12:58:15.526628Z",
    "operator_urls": [
      "https://www.jrkyushu-kippu.jp/fare/ticket/14"
    ],
    "operator_status": "unsupported_operator",
    "operator_http_status": null,
    "operator_observed_at": null
  }
]

JR West linked source: https://www.toretabi.jp/ticket/west_050.html -> https://tickets.jr-odekake.net/shohindb/view/consumer/tokutoku/detail.html?shnId=119000610; ordinary get observed 2026-10-04T13:00:00.664885Z; operator result unsupported_operator.

Linked Shikoku source https://www.toretabi.jp/ticket/shikoku_030.html supplies https://www.jr-eki.com/ticket/brand/2-2BW; official page identifies JR四国ツアー and copyright Shikoku Railway Company. Linked Kyushu source https://www.toretabi.jp/ticket/kyushu_013.html supplies https://www.jrkyushu-kippu.jp/fare/ticket/14; official title identifies JR九州 and corporate links. Both ordinary public HTML pages were independently read; initial unsupported statuses were before allowlist correction. Corporate homepage hosts not actually used by these detail links were removed from runtime allowlist. Formula/route-dependent ticket-set prices are left unknown rather than flattened from examples.

## Product extraction scope
Ordinary public HTML layout research binds JR Central section.main, JR Hokkaido section.detail-ticketSec, JR East section.contentsWrapper under main#contents with its dedicated #mainVisual heading, JR West div.ticketLayout__main with its ticketArticle heading, Shikoku div.dtl with its adjacent div.vi heading, and Kyushu #jkContents with #jkContainer heading. Rule/fare/channel/exception/PDF extraction reads only that uniquely bound product scope. Navigation, aside, related/recommended panels and foreign product articles are excluded; absent or ambiguous scopes produce unknown fields. JR East ordinary deployed-shape Go GET200 on this follow-up research supplied its actual layout; ordinary Python read403 is retained as a transport observation, not bypassed or interpreted as closure. Raw HTML layouts remain in private-captures, not manuscript publication.
