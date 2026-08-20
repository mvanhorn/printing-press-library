import test from "node:test";
import assert from "node:assert/strict";
import { mapWithConcurrency } from "../src/concurrency.js";

test("mapWithConcurrency returns results in input order regardless of completion order", async () => {
  const items = [40, 10, 30, 0, 20];
  const out = await mapWithConcurrency(items, 3, async (ms, i) => {
    await new Promise((resolve) => setTimeout(resolve, ms));
    return `${i}:${ms}`;
  });

  assert.deepEqual(out, ["0:40", "1:10", "2:30", "3:0", "4:20"]);
});

test("mapWithConcurrency never exceeds the limit of in-flight calls", async () => {
  let inFlight = 0;
  let maxInFlight = 0;
  const items = Array.from({ length: 12 }, (_, i) => i);

  await mapWithConcurrency(items, 4, async (n) => {
    inFlight++;
    maxInFlight = Math.max(maxInFlight, inFlight);
    await new Promise((resolve) => setTimeout(resolve, 5));
    inFlight--;
    return n;
  });

  assert.equal(maxInFlight, 4);
});

test("mapWithConcurrency handles empty input without invoking fn", async () => {
  let calls = 0;
  const out = await mapWithConcurrency([], 4, async () => {
    calls++;
    return 1;
  });

  assert.deepEqual(out, []);
  assert.equal(calls, 0);
});

test("mapWithConcurrency runs fewer workers than the limit when items are scarce", async () => {
  let inFlight = 0;
  let maxInFlight = 0;

  await mapWithConcurrency([1, 2], 8, async (n) => {
    inFlight++;
    maxInFlight = Math.max(maxInFlight, inFlight);
    await new Promise((resolve) => setTimeout(resolve, 5));
    inFlight--;
    return n;
  });

  assert.equal(maxInFlight, 2);
});
