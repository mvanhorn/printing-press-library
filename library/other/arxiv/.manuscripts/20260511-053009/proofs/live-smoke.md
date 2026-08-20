# arXiv live smoke

Commands exercised:

```bash
arxiv-pp-cli query --search-query 'cat:cs.AI' --start 0 --max-results 1 --sort-by submittedDate --sort-order descending --json --data-source live
arxiv-pp-cli query --id-list 1706.03762 --max-results 1 --json --data-source live
```

Result: PASS. The CLI reached `https://export.arxiv.org/api/query` and parsed the Atom feed into JSON entries.
