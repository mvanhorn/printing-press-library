# Send-check stable contact IDs

Preserve fail-closed send checks by rejecting live audience rows without a stable non-empty contact ID before tag merging or de-duplication. Regression coverage verifies every supported live selector returns a partial, API-class inconclusive result and never proceeds to DNC evaluation.
