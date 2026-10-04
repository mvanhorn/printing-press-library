# Historical representative generation measurements

These 27 real fresh-process wall-clock samples were measured during generation before the publication-only allocation patch. They describe the observed small-response generation build, not a new benchmark of the final publication binary and not an SLA. Three samples per workflow used semantic assertions; expected request counts derive from source, not tracing.

|Workflow|Median ms|Range ms|Expected requests|
|---|---:|---:|---:|
|catalog|7.647|7.362–8.286|0|
|required facilities|516.365|488.315–520.929|1|
|station|491.097|488.187–674.034|1|
|readiness|476.453|474.869–478.362|1|
|compare 2 IDs|695.807|691.282–711.566|2|
|nearby|502.470|493.225–513.648|1|
|station notices|5568.245|5479.023–5580.298|12|
|snapshot 2 IDs|682.124|681.036–694.533|2|
|offline changes|11.124|10.252–18.030|0|

The saved-file diff used a labeled controlled local mutation rather than claiming a real service change. All timing details remain private generation evidence; no raw provider outputs are packaged here.
