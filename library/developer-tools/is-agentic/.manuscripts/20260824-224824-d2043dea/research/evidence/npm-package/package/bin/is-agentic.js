#!/usr/bin/env node

const API_ORIGIN =
  process.env.IS_AGENTIC_API_ORIGIN || "https://is-agentic.com";
const CLI_VERSION = "1.0.1";
const BASE_COMMAND = "npx is-agentic";

const ANSI = {
  bold: 1,
  dim: 2,
  red: 31,
  green: 32,
  yellow: 33,
  cyan: 36,
  gray: 90,
};

function supportsColor(stream) {
  return Boolean(
    stream.isTTY &&
      !process.env.NO_COLOR &&
      process.env.TERM !== "dumb" &&
      process.env.FORCE_COLOR !== "0",
  );
}

function paint(value, codes, enabled) {
  if (!enabled) return String(value);
  return `\u001b[${codes.join(";")}m${value}\u001b[0m`;
}

function titleCase(value) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : value;
}

function formatNumber(value) {
  return Number.isInteger(value) ? String(value) : String(Number(value.toFixed(1)));
}

function terminalWidth() {
  return Math.max(56, Math.min(100, process.stdout.columns || 88));
}

function wrapWords(value, width) {
  const words = String(value).trim().replace(/\s+/g, " ").split(" ");
  const lines = [];
  let line = "";

  for (const word of words) {
    if (!line) {
      line = word;
    } else if (line.length + word.length + 1 <= width) {
      line += ` ${word}`;
    } else {
      lines.push(line);
      line = word;
    }
  }

  if (line) lines.push(line);
  return lines.length ? lines : [""];
}

function labeledParagraph(label, value, width, color) {
  const indent = "   ";
  const labelWidth = 10;
  const plainLabel = label.padEnd(labelWidth);
  const available = Math.max(24, width - indent.length - labelWidth);
  const wrapped = wrapWords(value, available);
  const visibleLabel = paint(plainLabel, [ANSI.bold], color);
  const continuation = " ".repeat(labelWidth);

  return wrapped
    .map(
      (line, index) =>
        `${indent}${index === 0 ? visibleLabel : continuation}${line}`,
    )
    .join("\n");
}

function scoreColor(score) {
  if (score === null) return ANSI.gray;
  if (score >= 80) return ANSI.green;
  if (score >= 60) return ANSI.yellow;
  return ANSI.red;
}

function scoreBar(score, width) {
  const filled =
    score === null
      ? 0
      : Math.max(
          0,
          Math.min(width, Math.round((Math.min(100, score) / 100) * width)),
        );
  return `${"█".repeat(filled)}${"░".repeat(width - filled)}`;
}

function colorScoreBar(line, color) {
  if (!color) return line;
  return line
    .replace(/█+/g, (segment) => paint(segment, [ANSI.bold], true))
    .replace(/░+/g, (segment) => paint(segment, [ANSI.gray, ANSI.dim], true));
}

function scoreGraphic(report, failedCount, partialCount, width, color) {
  const barWidth = Math.max(20, Math.min(36, width - 28));
  const bar = scoreBar(report.score, barWidth);
  const spacer = " ".repeat(barWidth);
  const scoreValue =
    report.score === null ? "— / 100" : `${formatNumber(report.score)} / 100`;
  const score = paint(
    scoreValue,
    [ANSI.bold, scoreColor(report.score)],
    color,
  );

  return [
    `  ${colorScoreBar(bar, color)}    ${score}`,
    `  ${spacer}    ${report.score_label}`,
    `  ${spacer}    ${failedCount} failed · ${partialCount} partial`,
  ].join("\n");
}

function breakdownRow(label, points, checks) {
  return `  ${label.padEnd(14)}${points.padStart(11)}    ${checks}`;
}

function findingBlock(issue, index, width, color) {
  const failed = issue.result === "failed";
  const result = failed ? "FAIL" : "PARTIAL";
  const resultCode = failed ? ANSI.red : ANSI.yellow;
  const marker = paint(result, [ANSI.bold, resultCode], color);
  const tier = paint(titleCase(issue.tier).toUpperCase(), [ANSI.bold], color);
  const title = paint(issue.name, [ANSI.bold], color);
  const lines = [`${index}. ${marker} · ${tier}  ${title}`];

  if (issue.details) {
    lines.push(labeledParagraph("Evidence", issue.details, width, color));
  }
  if (issue.recommendation) {
    lines.push(labeledParagraph("Fix", issue.recommendation, width, color));
  }

  return lines.join("\n");
}

function formatReport(report) {
  const color = supportsColor(process.stdout);
  const width = terminalWidth();
  const essential = report.score_breakdown.essential;
  const recommended = report.score_breakdown.recommended;
  const bonus = report.score_breakdown.bonus;
  const failed = report.issues.filter((issue) => issue.result === "failed");
  const partial = report.issues.filter((issue) => issue.result === "partial");
  const lines = [
    `${paint("▲ / Is Agentic", [ANSI.bold], color)}  ${paint(report.display_target, [ANSI.dim], color)}`,
    "",
    scoreGraphic(report, failed.length, partial.length, width, color),
  ];

  lines.push(
    "",
    paint("SCORE BREAKDOWN", [ANSI.bold], color),
    breakdownRow(
      "Essential",
      `${formatNumber(essential.earned)} / ${formatNumber(essential.available)}`,
      `${essential.passing} / ${essential.total} passed`,
    ),
    breakdownRow(
      "Recommended",
      `${formatNumber(recommended.earned)} / ${formatNumber(recommended.available)}`,
      `${recommended.passing} / ${recommended.total} passed`,
    ),
    breakdownRow(
      "Bonus",
      `+${formatNumber(bonus.points)}`,
      `${bonus.positive_signals} positive signals`,
    ),
  );

  if (!report.issues.length) {
    lines.push(
      "",
      paint("FINDINGS", [ANSI.bold], color),
      paint("  No failed or partial checks.", [ANSI.green], color),
    );
  } else {
    let issueNumber = 1;
    if (failed.length) {
      lines.push(
        "",
        paint(
          `FAILURES (${failed.length})`,
          [ANSI.bold, ANSI.red],
          color,
        ),
      );
      for (const issue of failed) {
        lines.push("", findingBlock(issue, issueNumber, width, color));
        issueNumber += 1;
      }
    }

    if (partial.length) {
      lines.push(
        "",
        paint(
          `PARTIAL (${partial.length})`,
          [ANSI.bold, ANSI.yellow],
          color,
        ),
      );
      for (const issue of partial) {
        lines.push("", findingBlock(issue, issueNumber, width, color));
        issueNumber += 1;
      }
    }
  }

  lines.push(
    "",
    paint("REPORT", [ANSI.bold], color),
    `  URL      ${report.report_url}`,
    `  Scanned  ${report.scanned_at}`,
    `  Checks   ${report.eligible_checks} eligible`,
  );

  return `${lines.join("\n")}\n`;
}

function usage() {
  return `Usage: ${BASE_COMMAND} <domain-or-url> [--json]

Retrieve the latest Is Agentic report, scanning the site when none exists.

Options:
  --json, -j  Print the unchanged API response as JSON
  --help, -h  Show this help
`;
}

function localProblem({ title, code, detail, resolution, status = 400 }) {
  return {
    type: "about:blank",
    title,
    status,
    detail,
    code,
    resolution,
  };
}

function formatProblem(problem) {
  const color = supportsColor(process.stderr);
  const code = problem.code ? ` [${problem.code}]` : "";
  const lines = [
    paint(`Error: ${problem.title}${code}`, [ANSI.bold, ANSI.red], color),
  ];
  if (problem.detail) lines.push(problem.detail);
  if (problem.resolution) lines.push("", `Next: ${problem.resolution}`);
  return `${lines.join("\n")}\n`;
}

function writeProblem(problem, json) {
  if (json) {
    process.stdout.write(`${JSON.stringify(problem, null, 2)}\n`);
  } else {
    process.stderr.write(formatProblem(problem));
  }
  process.exitCode = 1;
}

class CliProblem extends Error {
  constructor(problem) {
    super(problem.title);
    this.name = "CliProblem";
    this.problem = problem;
  }
}

function wait(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function requestReport(target) {
  const endpoint = new URL("/api/v1/report", API_ORIGIN);
  endpoint.searchParams.set("url", target);
  const response = await fetch(endpoint, {
    headers: {
      Accept: "application/json",
      "User-Agent": `is-agentic-cli/${CLI_VERSION}`,
    },
  });

  try {
    return { body: await response.json(), response };
  } catch {
    throw new CliProblem(
      localProblem({
        title: "Invalid API response",
        code: "invalid_api_response",
        status: 502,
        detail: `The Is Agentic API returned HTTP ${response.status} without a JSON body.`,
        resolution: "Retry shortly or open https://is-agentic.com/docs#errors.",
      }),
    );
  }
}

function scanProgress(target, json) {
  const enabled = !json;
  const interactive = Boolean(process.stderr.isTTY);
  const safeTarget = String(target).replace(/[\u0000-\u001f\u007f]/g, " ");

  function line(message) {
    if (!enabled || !interactive) return;
    process.stderr.write(`\r\u001b[2K${message}`);
  }

  return {
    start() {
      if (!enabled) return;
      if (interactive) line(`Scanning ${safeTarget}…`);
      else process.stderr.write(`No completed report. Scanning ${safeTarget}…\n`);
    },
    update(message) {
      const safeMessage = String(message)
        .replace(/[\u0000-\u001f\u007f]/g, " ")
        .slice(0, 64);
      line(`Scanning ${safeTarget} · ${safeMessage}`);
    },
    complete() {
      if (!enabled) return;
      if (interactive) {
        line(`Scan complete for ${safeTarget}.`);
        process.stderr.write("\n");
      } else {
        process.stderr.write("Scan complete.\n");
      }
    },
    clear() {
      if (enabled && interactive) process.stderr.write("\r\u001b[2K");
    },
  };
}

function ssePayload(frame) {
  const data = frame
    .split("\n")
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice(5).trimStart())
    .join("\n");
  if (!data) return null;
  try {
    return JSON.parse(data);
  } catch {
    return null;
  }
}

async function startMissingScan(target, json) {
  const progress = scanProgress(target, json);
  progress.start();

  try {
    const endpoint = new URL("/api/scan/stream", API_ORIGIN);
    endpoint.searchParams.set("target", target);
    const response = await fetch(endpoint, {
      headers: {
        Accept: "text/event-stream",
        "Cache-Control": "no-store",
        "User-Agent": `is-agentic-cli/${CLI_VERSION}`,
      },
    });

    if (!response.ok || !response.body) {
      await response.text();
      const retryAfter = response.headers.get("retry-after");
      throw new CliProblem(
        localProblem({
          title: "Is Agentic could not start the scan",
          code: "scan_start_failed",
          status: response.status || 502,
          detail: `The Is Agentic scan service returned HTTP ${response.status}.`,
          resolution: retryAfter
            ? `Retry after ${retryAfter} seconds.`
            : "Retry shortly or start the scan at https://is-agentic.com.",
        }),
      );
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let completed = false;
    let archived = false;
    let completedChecks = 0;
    let totalChecks = 0;

    function handleFrame(frame) {
      const event = ssePayload(frame);
      if (!event || typeof event !== "object") return;

      if (event.type === "scan_init") {
        totalChecks = Array.isArray(event.checkRoster)
          ? event.checkRoster.length
          : totalChecks;
      } else if (event.type === "check_start" && event.checkName) {
        progress.update(event.checkName);
      } else if (event.type === "check_complete") {
        completedChecks += 1;
        progress.update(
          totalChecks
            ? `${completedChecks}/${totalChecks} checks`
            : `${completedChecks} checks`,
        );
      } else if (event.type === "scan_complete") {
        completed = true;
        progress.update("Finalizing report");
      } else if (event.type === "scan_archived") {
        archived = true;
      } else if (event.type === "error") {
        throw new CliProblem(
          localProblem({
            title: "Is Agentic could not complete the scan",
            code: "scan_failed",
            status: 502,
            detail:
              "The Is Agentic scan service did not produce a completed report for this target.",
            resolution: "Retry shortly or start the scan at https://is-agentic.com.",
          }),
        );
      }
    }

    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n");
        let boundary;
        while ((boundary = buffer.indexOf("\n\n")) !== -1) {
          handleFrame(buffer.slice(0, boundary));
          buffer = buffer.slice(boundary + 2);
        }
      }
      buffer += decoder.decode();
      if (buffer) handleFrame(buffer);
    } catch (error) {
      try {
        await reader.cancel();
      } catch {
        // The server may have already closed after its terminal error frame.
      }
      throw error;
    } finally {
      reader.releaseLock();
    }

    if (!completed && !archived) {
      throw new CliProblem(
        localProblem({
          title: "Is Agentic scan was interrupted",
          code: "scan_interrupted",
          status: 502,
          detail:
            "The Is Agentic scan stream ended before a completed report was available.",
          resolution: "Retry the command shortly.",
        }),
      );
    }

    progress.complete();
  } catch (error) {
    progress.clear();
    throw error;
  }
}

async function readReportAfterScan(target) {
  let result = null;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    result = await requestReport(target);
    if (result.response.ok || result.body?.code !== "report_not_found") {
      return result;
    }
    await wait(250 * (attempt + 1));
  }

  throw new CliProblem(
    localProblem({
      title: "Is Agentic report is not available yet",
      code: "scan_not_archived",
      status: 503,
      detail:
        "The Is Agentic scan finished, but its public report could not be retrieved.",
      resolution: "Retry the command shortly.",
    }),
  );
}

function parseArguments(args) {
  const json = args.includes("--json") || args.includes("-j");
  let help = false;
  let optionsEnded = false;
  let target = null;
  let problem = null;

  for (const argument of args) {
    if (!optionsEnded && argument === "--") {
      optionsEnded = true;
    } else if (!optionsEnded && (argument === "--json" || argument === "-j")) {
      // The first pass already records JSON mode.
    } else if (!optionsEnded && (argument === "--help" || argument === "-h")) {
      help = true;
    } else if (!optionsEnded && argument.startsWith("-")) {
      problem = localProblem({
        title: "Unknown option",
        code: "unknown_option",
        detail: `The CLI does not recognize ${argument}.`,
        resolution: `Run ${BASE_COMMAND} --help to see supported options.`,
      });
      break;
    } else if (target !== null) {
      problem = localProblem({
        title: "Too many targets",
        code: "too_many_targets",
        detail: "The CLI accepts one domain or URL at a time.",
        resolution: `Run ${BASE_COMMAND} <domain-or-url> once for each target.`,
      });
      break;
    } else {
      target = argument;
    }
  }

  return { help, json, problem, target };
}

const options = parseArguments(process.argv.slice(2));

if (options.help && !options.problem) {
  process.stdout.write(usage());
} else if (options.problem) {
  writeProblem(options.problem, options.json);
} else if (!options.target) {
  if (options.json) {
    writeProblem(
      localProblem({
        title: "Target required",
        code: "target_required",
        detail: "No domain or URL was provided.",
        resolution: `Run ${BASE_COMMAND} <domain-or-url> --json.`,
      }),
      true,
    );
  } else {
    process.stderr.write(usage());
    process.exitCode = 1;
  }
} else {
  try {
    let result = await requestReport(options.target);
    if (!result.response.ok && result.body?.code === "report_not_found") {
      await startMissingScan(options.target, options.json);
      result = await readReportAfterScan(options.target);
    }

    if (!result.response.ok) {
      writeProblem(result.body, options.json);
    } else if (options.json) {
      process.stdout.write(`${JSON.stringify(result.body, null, 2)}\n`);
    } else {
      process.stdout.write(formatReport(result.body));
    }
  } catch (error) {
    writeProblem(
      error instanceof CliProblem
        ? error.problem
        : localProblem({
            title: "Could not reach the Is Agentic API",
            code: "api_unreachable",
            status: 503,
            detail: "The CLI could not connect to the public report API.",
            resolution: "Check your network connection and retry.",
          }),
      options.json,
    );
  }
}
