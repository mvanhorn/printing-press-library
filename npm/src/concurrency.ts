/**
 * Run `fn` over `items` with at most `limit` calls in flight at once, returning
 * results in input order. A worker pool pulls from a shared cursor, so a slow
 * item never blocks the others from starting — wall-clock is bounded by the
 * slowest `limit`-sized window, not the serial sum. `limit` is clamped to
 * `[1, items.length]`.
 *
 * `fn` is expected to resolve (callers that must not abort the batch on a single
 * failure should catch inside `fn` and return a sentinel); a rejection here
 * propagates and rejects the whole call.
 */
export async function mapWithConcurrency<T, R>(
  items: readonly T[],
  limit: number,
  fn: (item: T, index: number) => Promise<R>,
): Promise<R[]> {
  const results = new Array<R>(items.length);
  let cursor = 0;

  const worker = async (): Promise<void> => {
    while (true) {
      const index = cursor++;
      if (index >= items.length) {
        return;
      }
      results[index] = await fn(items[index]!, index);
    }
  };

  const workerCount = Math.min(Math.max(1, limit), items.length);
  await Promise.all(Array.from({ length: workerCount }, worker));
  return results;
}
