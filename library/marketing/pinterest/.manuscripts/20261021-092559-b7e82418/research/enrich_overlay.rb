# Idempotent overlay enrichment for the pinterest reprint.
# Usage: ruby enrich_overlay.rb <base-openapi.yaml> <tools-manifest.json> <out-overlay.yaml>
#
# - Required non-opaque params get valid examples (enum-safe) so help Examples synthesize.
# - Opaque ID params (including ad_account_id) carry no example; their
#   operations get an x-pp-example with `example-value` placeholders so live dogfood
#   resolves real IDs from list companions / the synced store, or records
#   blocked-fixture instead of 404ing on a fabricated ID.
# - Body-only / mutating operations get a --dry-run x-pp-example.
# - Account-state gated endpoints get x-live-dogfood-requires-tier.
require "yaml"
require "json"

base, manifest, out = ARGV
d = YAML.load_file(base)
P = d["components"]["parameters"]
SCHEMAS = d["components"]["schemas"]
tools = JSON.parse(File.read(manifest))["tools"]
cmd_for = {}
tools.each { |t| cmd_for[[t["method"].upcase, t["path"]]] = t["name"].tr("_", " ") }

SAMPLE_ID = "1234567890123456789"
EXAMPLES = {
  "start_date" => "2026-10-01", "end_date" => "2026-10-14", "granularity" => "DAY",
  "columns" => "SPEND_IN_DOLLAR", "region" => "US", "country_code" => "US",
  "username" => "pinterest", "metric_types" => "IMPRESSION", "date" => "2026-10-14",
  "is_active" => "true", "keywords" => "meal-prep", "term" => "meal-prep", "terms" => "meal-prep",
  "query" => "meal-prep",
}

def resolve(p)
  p["$ref"] ? P[p["$ref"].split("/").last] : p
end

def opaque?(q)
  n = q["name"].to_s
  s = q["schema"] || {}
  n == "id" || n.end_with?("_id", "_ids") || n.include?("token") || s["pattern"] == "^\\d+$"
end

def example_for(q)
  s = q["schema"] || {}
  return s["default"] if s.key?("default")
  s = SCHEMAS[s["$ref"].split("/").last] || s if s["$ref"]
  items = s["items"] || {}
  items = SCHEMAS[items["$ref"].split("/").last] || items if items["$ref"]
  enum = s["enum"] || items["enum"]
  return enum.first if enum && !enum.empty?
  EXAMPLES[q["name"]]
end

# 1. Parameter examples: valid for non-opaque, removed for opaque.
param_objs = P.values.dup
d["paths"].each_value do |item|
  item.each do |m, op|
    next unless op.is_a?(Hash) && op["parameters"]
    op["parameters"].each { |p| param_objs << p unless p["$ref"] }
  end
end
param_objs.each do |q|
  next unless q["required"]
  if opaque?(q)
    q.delete("example"); (q["schema"] || {}).delete("example")
  elsif (v = example_for(q))
    q["example"] = v
  end
end

GATED = {
  "catalog-owner" => %w[/catalogs/available_filter_values /catalogs/{catalog_id}/local_stores /catalogs/product_groups/{product_group_id}/products
                        /catalogs/product_groups/{product_group_id} /catalogs/product_groups /catalogs/product_groups/{product_group_id}/product_counts
                        /catalogs/{catalog_id}/supplemental_items/batch/{request_id}],
  "restricted-labels" => %w[/ad_accounts/{ad_account_id}/labels],
  "catalog-owner-stats" => %w[/catalogs/reports/stats],
}
FIXTURE_FLAGS = {
  ["GET", "/resources/metrics_ready_state"] => "--date=example-value",
  ["GET", "/ad_accounts/{ad_account_id}/ads/analytics"] => "--ad-ids=example-value",
  ["GET", "/ad_accounts/{ad_account_id}/product_group_promotions"] => "--product-group-promotion-ids=example-value",
}

stats = Hash.new(0)
d["paths"].each do |path, item|
  item.each do |m, op|
    next unless %w[get post put patch delete].include?(m) && op.is_a?(Hash)
    method = m.upcase
    GATED.each { |tier, paths| (op["x-live-dogfood-requires-tier"] = tier; stats[:tier] += 1) if method == "GET" && paths.include?(path) }
    if (fx = FIXTURE_FLAGS[[method, path]])
      op["x-happy-args"] = fx; stats[:happy] += 1
    end
    cmd = cmd_for[[method, path]] or next
    params = (op["parameters"] || []).map { |p| resolve(p) }.compact
    req = params.select { |q| q["required"] }
    path_params = path.scan(/\{([^}]+)\}/).flatten
    has_opaque = req.any? { |q| opaque?(q) }
    has_body = op.dig("requestBody", "required")
    next unless has_opaque || has_body || method != "GET"
    # Display IDs are samples: with no explicit happy-args positional, live dogfood
    # re-resolves id-shaped positionals from list companions / the synced store.
    # (The generator drops Example lines containing the literal example-value.)
    args = path_params.map { SAMPLE_ID }
    blocked = nil
    req.select { |q| q["in"] == "query" }.each do |q|
      v = opaque?(q) ? SAMPLE_ID : example_for(q)
      next unless v
      flag = "--#{q["name"].tr("_", "-")}"
      blocked ||= "#{flag}=example-value" if opaque?(q)
      args << flag << v.to_s
    end
    if blocked && !op["x-happy-args"]
      op["x-happy-args"] = blocked
      stats[:happy] += 1
    end
    args << "--dry-run" if method != "GET"
    op["x-pp-example"] = (["pinterest-pp-cli", cmd] + args).join(" ")
    stats[:example] += 1
  end
end

File.write(out, d.to_yaml(line_width: -1))
warn "enriched: #{stats.inspect}"

stats_op = d.dig("paths", "/catalogs/reports/stats", "get")
stats_op["x-pp-example"] = %q(pinterest-pp-cli catalogs reports-stats --parameters '{"catalog_id":"1234567890123456789","report_type":"FEED_INGESTION_ISSUES"}') if stats_op
File.write(out, d.to_yaml(line_width: -1))
