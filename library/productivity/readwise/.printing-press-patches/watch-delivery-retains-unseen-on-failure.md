# At-least-once watch delivery

Keep items unseen when no matching receiver succeeds, including an empty receiver set. Retry a failed POST once with 100 ms cancellable backoff. Strict mode must fail, and payload construction errors must retain unseen IDs too. A partially successful fan-out advances the batch per the accepted delivery contract; receivers should deduplicate item IDs.

Round 2: any matching hook failure after retry returns an error, including partial multi-recipient delivery. Watch strict handling then retains all items for at-least-once retry. Failure lines identify hook ids without full URLs.
